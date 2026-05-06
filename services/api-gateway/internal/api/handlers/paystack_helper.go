package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/util"
)

func (h *RouteHandler) sendApiRequest(ctx context.Context, method, path string, payload io.Reader) ([]byte, error) {
	// Configure request details
	req, err := http.NewRequestWithContext(
		ctx, method,
		h.cfg.Env.PaystackApiUrl+path,
		payload,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to build http request")
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.cfg.Env.PaystackSecretKey)

	// Send request
	response, err := h.cfg.HttpClient.Do(req)
	if err != nil {
		return nil, util.ErrApiRequestFailure
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusInternalServerError {
		return nil, util.ErrGatewayUnavailable
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read response body from paystack api")
		return nil, err
	}

	return body, nil
}

func (h *RouteHandler) createTransferRecipient(ctx context.Context, name string, details *contracts.AccountDetails) (string, error) {
	const cacheKey = "paystack:banks:nigeria"
	var banksList contracts.PaystackBankResponse

	// Try to get bank list from cache
	cachedBanks, err := h.cfg.Cache.Get(ctx, cacheKey).Result()
	if err == nil {
		if err := json.Unmarshal([]byte(cachedBanks), &banksList); err == nil {
			goto findBankCode
		}
	}

	// Fetch from Paystack API if not in cache
	{
		banksListResp, err := h.sendApiRequest(
			ctx,
			"GET",
			"/bank?country=nigeria&use_cursor=false&perPage=100",
			nil,
		)
		if err != nil {
			return "", err
		}

		if err := json.Unmarshal(banksListResp, &banksList); err != nil {
			log.Error().Err(err).Msg("Failed to unmarshal paystack banks list response")
			return "", err
		}

		// Cache the response for 30 days
		if banksList.Status {
			h.cfg.Cache.Set(ctx, cacheKey, banksListResp, 30*24*time.Hour)
		}
	}

findBankCode:
	var bankCode string
	for _, bank := range banksList.Data {
		if bank.Name == details.BankName {
			bankCode = bank.Code
			break
		}
	}

	if bankCode == "" {
		return "", util.ErrUnsupportedBank
	}

	// Verify account details
	acctVerifcationResp, err := h.sendApiRequest(
		ctx,
		"GET",
		fmt.Sprintf("/bank/resolve?account_number=%s&bank_code=%s", details.AccountNumber, bankCode),
		nil,
	)
	if err != nil {
		return "", err
	}

	var verificationInfo contracts.AccountVerificationResponse
	if err := json.Unmarshal(acctVerifcationResp, &verificationInfo); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal paystack account verification response")
		return "", err
	}

	if !strings.EqualFold(verificationInfo.Data.AccountName, details.AccountName) {
		return "", util.ErrAccountNameMismatch
	}

	payload, err := json.Marshal(contracts.CreateTransferRecipientPayload{
		Type:          "nuban",
		Name:          name,
		AccountNumber: details.AccountNumber,
		BankCode:      bankCode,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal paystack transfer recipient request payload")
		return "", err
	}

	trasnferRecipientResp, err := h.sendApiRequest(
		ctx,
		"POST",
		"/transferrecipient",
		bytes.NewBuffer(payload),
	)
	if err != nil {
		return "", err
	}

	var trasnferRecipient contracts.TransferRecipientResponse
	if err := json.Unmarshal(trasnferRecipientResp, &trasnferRecipient); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal paystack transfer recipient response")
		return "", err
	}

	return trasnferRecipient.Data.RecipientCode, nil
}
