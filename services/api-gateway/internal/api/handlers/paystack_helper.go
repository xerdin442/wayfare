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

	"github.com/xerdin442/wayfare/shared/contracts"
)

func (h *RouteHandler) sendApiRequest(ctx context.Context, method, path string, payload io.Reader) ([]byte, error) {
	// Configure request details
	req, err := http.NewRequestWithContext(
		ctx, method,
		h.cfg.Env.PaystackApiUrl+path,
		payload,
	)
	if err != nil {
		return nil, fmt.Errorf("Error configuring new HTTP request: %s", err.Error())
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.cfg.Env.PaystackSecretKey)

	// Send request
	response, err := h.cfg.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error sending request to Paystack API")
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read Paystack API response body: %s", err.Error())
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
			return "", fmt.Errorf("Failed to unmarshal response from Paystack API: %v", err)
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
		return "", fmt.Errorf("Wayfare does not support payouts to %s", details.BankName)
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
		return "", fmt.Errorf("Failed to unmarshal response from Paystack API: %v", err)
	}

	if !strings.EqualFold(verificationInfo.Data.AccountName, details.AccountName) {
		return "", fmt.Errorf("Account name does not match. Please check the spelling or order of your account name")
	}

	payload, err := json.Marshal(contracts.CreateTransferRecipientPayload{
		Type:          "nuban",
		Name:          name,
		AccountNumber: details.AccountNumber,
		BankCode:      bankCode,
	})
	if err != nil {
		return "", fmt.Errorf("Failed to marshal transfer_recipient request payload: %s", err.Error())
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
		return "", fmt.Errorf("Failed to unmarshal response from Paystack API: %v", err)
	}

	return trasnferRecipient.Data.RecipientCode, nil
}
