package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	repo "github.com/xerdin442/wayfare/services/payment/internal/infra/repository"
	"github.com/xerdin442/wayfare/services/payment/internal/secrets"
	"github.com/xerdin442/wayfare/shared/contracts"
	rpc "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrGatewayUnavailable = errors.New("payment gateway is currently unavailable")

type PaymentService struct {
	rpc.UnimplementedPaymentServiceServer
	repo  *repo.PaymentRepository
	cache *redis.Client
	env   *secrets.Secrets
}

func NewPaymentService(r *repo.PaymentRepository, c *redis.Client, s *secrets.Secrets) *PaymentService {
	return &PaymentService{
		repo:  r,
		cache: c,
		env:   s,
	}
}

func (s *PaymentService) sendApiRequest(url, secretKey string, payload io.Reader) ([]byte, error) {
	// Configure request details
	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return nil, fmt.Errorf("Error configuring new HTTP request. %s", err.Error())
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+secretKey)

	// Send request to payment provider
	httpClient := &http.Client{Timeout: 30 * time.Second}
	response, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error sending request to payment provider. %s", err.Error())
	}
	defer response.Body.Close()

	if response.StatusCode >= 500 {
		return nil, ErrGatewayUnavailable
	}

	body, _ := io.ReadAll(response.Body)
	return body, nil
}

func (s *PaymentService) generatePaystackCheckout(req *contracts.PaystackCheckoutRequest) (string, error) {
	payload, _ := json.Marshal(req)
	httpResp, err := s.sendApiRequest(s.env.PaystackApiUrl, s.env.PaystackSecretKey, bytes.NewBuffer(payload))
	if err != nil {
		return "", err
	}

	var checkoutInfo contracts.PaystackCheckoutResponse
	if err := json.Unmarshal(httpResp, &checkoutInfo); err != nil {
		return "", fmt.Errorf("Failed to unmarshal response from Paystack: %v", err)
	}

	return checkoutInfo.Data.AuthorizationUrl, nil
}

func (s *PaymentService) generateFlutterwaveCheckout(req *contracts.FlutterwaveCheckoutRequest) (string, error) {
	payload, _ := json.Marshal(req)
	httpResp, err := s.sendApiRequest(s.env.FlutterwaveApiUrl, s.env.FlutterwaveSecretKey, bytes.NewBuffer(payload))
	if err != nil {
		return "", err
	}

	var checkoutInfo contracts.FlutterwaveCheckoutResponse
	if err := json.Unmarshal(httpResp, &checkoutInfo); err != nil {
		return "", fmt.Errorf("Failed to unmarshal response from Flutterwave: %v", err)
	}

	return checkoutInfo.Data.Link, nil
}

func (s *PaymentService) InitiatePayment(ctx context.Context, req *rpc.InitiatePaymentRequest) (*rpc.InitiatePaymentResponse, error) {
	var checkoutUrl string

	// Store transaction details
	txnID, err := s.repo.CreateTransaction(ctx, req)
	if err != nil {
		return &rpc.InitiatePaymentResponse{}, status.Errorf(codes.Internal, "Database operation failed")
	}

	// Check health status of primary payment gateway
	gatewayStatusKey := "gateway:paystack:status"
	n, err := s.cache.Exists(ctx, gatewayStatusKey).Result()
	if err != nil {
		log.Error().Err(err).Msg("Error fetching gateway status from cache")
		return nil, status.Errorf(codes.Internal, "Error occurred during payment processing")
	}

	// Configure payloads for checkout request
	paystackRequestPayload := &contracts.PaystackCheckoutRequest{
		Email:       req.Email,
		Amount:      req.Amount,
		Reference:   txnID,
		Channels:    []string{"card", "apple_pay", "bank_transfer"},
		CallbackUrl: req.CustomRedirect,
	}

	flutterwaveRequestPayload := &contracts.FlutterwaveCheckoutRequest{
		Amount:      req.Amount,
		TxRef:       txnID,
		RedirectUrl: req.CustomRedirect,
		Customer: &contracts.FlutterwaveCustomer{
			Email: req.Email,
		},
	}

	if n > 0 {
		for i := range 3 {
			checkoutUrl, err = s.generateFlutterwaveCheckout(flutterwaveRequestPayload)
			if err == nil {
				break
			}

			if errors.Is(err, ErrGatewayUnavailable) {
				if i == 2 {
					log.Warn().Msg("All payment gateways are currently unavailable!")

					// Update transaction details
					if err := s.repo.UpdateTransaction(ctx, txnID, types.PaymentStatusAborted, types.ProviderFlutterwave); err != nil {
						return &rpc.InitiatePaymentResponse{}, status.Errorf(codes.Internal, "Database operation failed")
					}

					return nil, status.Errorf(codes.Unavailable, "Service unavailable")
				}

				log.Warn().Int("attempt", i+1).Msg("Flutterwave gateway is currently unavailable. Retrying...")
				continue
			} else {
				return nil, status.Errorf(codes.Internal, "Error occurred during payment processing")
			}
		}
	} else {
		for i := range 3 {
			checkoutUrl, err = s.generatePaystackCheckout(paystackRequestPayload)
			if err == nil {
				break
			}

			if errors.Is(err, ErrGatewayUnavailable) {
				if i == 2 {
					log.Warn().Msg("Paystack gateway is unavailable. Redirecting requests to backup gateway...")

					// Mark primary payment gateway as unavailable
					if err := s.cache.Set(ctx, gatewayStatusKey, "unavailable", 10*time.Minute).Err(); err != nil {
						log.Error().Err(err).Msg("Error setting gateway status")
					}

					// Redirect requests to backup payment gateway
					for j := range 3 {
						checkoutUrl, err = s.generateFlutterwaveCheckout(flutterwaveRequestPayload)
						if err == nil {
							break
						}

						if errors.Is(err, ErrGatewayUnavailable) {
							if j == 2 {
								log.Warn().Msg("All payment gateways are currently unavailable!")

								// Update transaction details
								if err := s.repo.UpdateTransaction(ctx, txnID, types.PaymentStatusAborted, types.ProviderFlutterwave); err != nil {
									return &rpc.InitiatePaymentResponse{}, status.Errorf(codes.Internal, "Database operation failed")
								}

								return nil, status.Errorf(codes.Unavailable, "Service unavailable")
							}

							log.Warn().Int("attempt", j+1).Msg("Flutterwave gateway is currently unavailable. Retrying...")
							continue
						} else {
							return nil, status.Errorf(codes.Internal, "Error occurred during payment processing")
						}
					}
				} else {
					log.Warn().Int("attempt", i+1).Msg("Paystack gateway is currently unavailable. Retrying...")
					continue
				}
			} else {
				return nil, status.Errorf(codes.Internal, "Error occurred during payment processing")
			}
		}
	}

	return &rpc.InitiatePaymentResponse{CheckoutUrl: checkoutUrl}, nil
}
