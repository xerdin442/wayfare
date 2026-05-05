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
	"github.com/shopspring/decimal"
	repo "github.com/xerdin442/wayfare/services/payment/internal/infra/repository"
	"github.com/xerdin442/wayfare/services/payment/internal/secrets"
	"github.com/xerdin442/wayfare/shared/analytics"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/models"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/tracing"
	"github.com/xerdin442/wayfare/shared/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrGatewayUnavailable = errors.New("payment gateway is currently unavailable")
	ErrApiRequestFailure  = errors.New("failed to send api request")
)

type PaymentService struct {
	pb.UnimplementedPaymentServiceServer
	repo       *repo.PaymentRepository
	cache      *redis.Client
	bus        messaging.MessageBus
	env        *secrets.Secrets
	httpClient *http.Client
}

func NewPaymentService(r *repo.PaymentRepository, b messaging.MessageBus, c *redis.Client, s *secrets.Secrets) *PaymentService {
	return &PaymentService{
		repo:       r,
		cache:      c,
		bus:        b,
		env:        s,
		httpClient: tracing.NewHttpClient(),
	}
}

func (s *PaymentService) sendApiRequest(ctx context.Context, url, secretKey string, payload io.Reader) ([]byte, error) {
	// Configure request details
	req, err := http.NewRequestWithContext(ctx, "POST", url, payload)
	if err != nil {
		return nil, fmt.Errorf("Error configuring new HTTP request. %s", err.Error())
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+secretKey)

	// Send request to payment provider
	response, err := s.httpClient.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("Error sending request to payment provider")
		return nil, ErrApiRequestFailure
	}
	defer response.Body.Close()

	if response.StatusCode >= 500 {
		errorBody, err := io.ReadAll(response.Body)
		if err != nil {
			log.Error().Str("provider_url", url).Err(err).Msg("Failed to read gateway error response")
			return nil, ErrGatewayUnavailable
		}

		var gatewayErr contracts.GatewayErrorResponse
		if err := json.Unmarshal(errorBody, &gatewayErr); err != nil {
			log.Error().Str("provider_url", url).Err(err).Msg("Failed to unmarshal gateway error response")
			return nil, ErrGatewayUnavailable
		}

		// Log error details for debugging
		log.Error().
			Int("http_status", response.StatusCode).
			Str("provider_url", url).
			Str("message", gatewayErr.Message).
			Str("code", gatewayErr.Code).
			Str("type", gatewayErr.Type).
			Str("error_id", gatewayErr.ErrorID).
			Msg("Gateway error response")

		return nil, ErrGatewayUnavailable
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read API response body: %s", err.Error())
	}

	return body, nil
}

func (s *PaymentService) isGatewayUnavailable(err error) bool {
	return errors.Is(err, ErrGatewayUnavailable) || errors.Is(err, ErrApiRequestFailure)
}

func (s *PaymentService) generatePaystackCheckout(ctx context.Context, req *contracts.PaystackCheckoutRequest) (string, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("Failed to marshal checkout request payload: %s", err.Error())
	}

	httpResp, err := s.sendApiRequest(
		ctx,
		fmt.Sprintf("%s/transaction/initialize", s.env.PaystackApiUrl),
		s.env.PaystackSecretKey,
		bytes.NewBuffer(payload),
	)
	if err != nil {
		return "", err
	}

	var checkoutInfo contracts.PaystackCheckoutResponse
	if err := json.Unmarshal(httpResp, &checkoutInfo); err != nil {
		return "", fmt.Errorf("Failed to unmarshal response from Paystack: %v", err)
	}

	return checkoutInfo.Data.AuthorizationUrl, nil
}

func (s *PaymentService) generateFlutterwaveCheckout(ctx context.Context, req *contracts.FlutterwaveCheckoutRequest) (string, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("Failed to marshal checkout request payload: %s", err.Error())
	}

	httpResp, err := s.sendApiRequest(
		ctx,
		fmt.Sprintf("%s/v3/payments", s.env.FlutterwaveApiUrl),
		s.env.FlutterwaveSecretKey,
		bytes.NewBuffer(payload),
	)
	if err != nil {
		return "", err
	}

	var checkoutInfo contracts.FlutterwaveCheckoutResponse
	if err := json.Unmarshal(httpResp, &checkoutInfo); err != nil {
		return "", fmt.Errorf("Failed to unmarshal response from Flutterwave: %v", err)
	}

	return checkoutInfo.Data.Link, nil
}

func (s *PaymentService) buildCheckoutPayloads(req *pb.InitiatePaymentRequest, txnID string) (
	*contracts.PaystackCheckoutRequest,
	*contracts.FlutterwaveCheckoutRequest,
	error,
) {
	metadata := &types.PaymentMetadata{
		TripID:       req.TripId,
		UserID:       req.UserId,
		TripRating:   req.TripRating,
		RiderComment: req.RiderComment,
		DriverTip:    req.DriverTip * 100,
	}

	paystackMetadata, err := json.Marshal(metadata)
	if err != nil {
		return nil, nil, err
	}

	paystackPayload := &contracts.PaystackCheckoutRequest{
		Email:       req.Email,
		Amount:      req.Amount,
		Reference:   txnID,
		Channels:    []string{"card", "apple_pay", "bank_transfer"},
		CallbackUrl: req.CustomRedirect,
		Metadata:    string(paystackMetadata),
	}

	flutterwavePayload := &contracts.FlutterwaveCheckoutRequest{
		Amount:      req.Amount / 100,
		TxRef:       txnID,
		RedirectUrl: req.CustomRedirect,
		Meta:        metadata,
		Customer: &contracts.FlutterwaveCustomer{
			Email: req.Email,
		},
	}

	return paystackPayload, flutterwavePayload, nil
}

func (s *PaymentService) resolvePaymentProvider(ctx context.Context, cacheKey string) types.PaymentProvider {
	n, err := s.cache.Exists(ctx, cacheKey).Result()
	if err != nil {
		log.Error().Err(err).Msg("Error fetching gateway status")
		return types.ProviderPaystack
	}

	if n > 0 {
		return types.ProviderFlutterwave
	}

	return types.ProviderPaystack
}

func (s *PaymentService) generateCheckoutUrl(
	ctx context.Context,
	provider types.PaymentProvider,
	paystackPayload *contracts.PaystackCheckoutRequest,
	flutterwavePayload *contracts.FlutterwaveCheckoutRequest,
) (string, error) {
	var url string
	var err error

	for i := range 3 {
		switch provider {
		case types.ProviderPaystack:
			url, err = s.generatePaystackCheckout(ctx, paystackPayload)

		case types.ProviderFlutterwave:
			url, err = s.generateFlutterwaveCheckout(ctx, flutterwavePayload)
		}

		if err == nil {
			break
		}

		if s.isGatewayUnavailable(err) {
			// Abort payment attempt if fallback gateway is also unavailble
			if i == 2 && provider == types.ProviderFlutterwave {
				log.Warn().Msg("All payment gateways are currently unavailable!")

				// Update transaction details
				if err := s.repo.UpdateTransaction(
					ctx,
					flutterwavePayload.TxRef,
					types.PaymentStatusAborted,
					types.ProviderFlutterwave,
				); err != nil {
					return "", err
				}

				// Update analytics
				tripEvent := &models.TripEventModel{
					TransactionRef:  flutterwavePayload.TxRef,
					PaymentProvider: types.ProviderFlutterwave,
					PaymentStatus:   types.PaymentStatusAborted,
					Amount:          decimal.NewFromInt(flutterwavePayload.Amount),
				}
				if err := analytics.SendEvent(ctx, s.bus, tripEvent); err != nil {
					return "", err
				}

				return "", err
			}

			log.Warn().Int("attempt", i+1).Msgf("%v gateway is currently unavailable. retrying...", provider)
			continue
		}
	}

	return url, err
}

func (s *PaymentService) InitiatePayment(ctx context.Context, req *pb.InitiatePaymentRequest) (*pb.InitiatePaymentResponse, error) {
	const gatewayStatusKey = "gateway:paystack:status"
	var checkoutUrl string
	var txnID string

	// Check for pending transaction from unfinished checkout session
	existingTxn, err := s.repo.GetTransactionByTripID(ctx, req.TripId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if existingTxn != nil {
		// Use existing transaction
		txnID = existingTxn.ID.Hex()
	} else {
		// Create new transaction
		txnID, err = s.repo.CreateTransaction(ctx, &repo.CreateTransactionData{
			TripID: req.TripId,
			Amount: req.Amount,
			Type:   types.TransactionCheckout,
		})
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	// Select payment provider
	provider := s.resolvePaymentProvider(ctx, gatewayStatusKey)

	// Configure checkout request payloads
	paystackPayload, flutterwavePayload, err := s.buildCheckoutPayloads(req, txnID)
	if err != nil {
		return nil, status.Error(codes.Internal, "Error building checkout payload")
	}

	// Generate checkout url
	checkoutUrl, err = s.generateCheckoutUrl(ctx, provider, paystackPayload, flutterwavePayload)
	if err != nil {
		if s.isGatewayUnavailable(err) && provider == types.ProviderPaystack {
			// Mark primary payment gateway as unavailable
			if err := s.cache.Set(ctx, gatewayStatusKey, "unavailable", 30*time.Minute).Err(); err != nil {
				log.Error().Err(err).Msg("Error setting gateway status")
			}

			// Retry payment attempt with fallback gateway
			checkoutUrl, err = s.generateCheckoutUrl(ctx, types.ProviderFlutterwave, nil, flutterwavePayload)

			if err != nil {
				// Notify the client that payment gateways are unavailable
				if s.isGatewayUnavailable(err) {
					return nil, status.Error(codes.Unavailable, "Payment gateway is currently unavailable")
				}

				return nil, status.Error(codes.Internal, "Error generating checkout url")
			}
		}

		return nil, status.Error(codes.Internal, "Error generating checkout url")
	}

	return &pb.InitiatePaymentResponse{CheckoutUrl: checkoutUrl}, nil
}
