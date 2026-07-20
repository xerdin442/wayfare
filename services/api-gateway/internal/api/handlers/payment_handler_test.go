package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xerdin442/wayfare/shared/types"
)

func TestHandlePaymentCallback_NoSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)

	bus := &mockBus{}
	h := buildTestHandler(t, bus)
	h.cfg.Env.PaystackSecretKey = "test-paystack-secret"

	router := gin.New()
	router.POST("/callback", h.HandlePaymentCallback)

	req := httptest.NewRequest("POST", "/callback", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandlePaymentCallback_InvalidPaystackSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"event":"charge.success","data":{"reference":"ref-123"}}`

	bus := &mockBus{}
	h := buildTestHandler(t, bus)
	h.cfg.Env.PaystackSecretKey = "test-paystack-secret"

	router := gin.New()
	router.POST("/callback", h.HandlePaymentCallback)

	req := httptest.NewRequest("POST", "/callback", strings.NewReader(body))
	req.Header.Set("x-paystack-signature", "invalid-hash-value")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandlePaymentCallback_InvalidEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"event":"invalid.event","data":{"reference":"ref-123"}}`
	sig := computeHMACSHA512("test-paystack-secret", body)

	bus := &mockBus{}
	h := buildTestHandler(t, bus)
	h.cfg.Env.PaystackSecretKey = "test-paystack-secret"

	router := gin.New()
	router.POST("/callback", h.HandlePaymentCallback)

	req := httptest.NewRequest("POST", "/callback", strings.NewReader(body))
	req.Header.Set("x-paystack-signature", sig)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandlePaymentCallback_InvalidFlutterwaveSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"event":"charge.completed","data":{"status":"successful"}}`

	bus := &mockBus{}
	h := buildTestHandler(t, bus)
	h.cfg.Env.FlutterwaveVerifHash = "test-flw-hash"

	router := gin.New()
	router.POST("/callback", h.HandlePaymentCallback)

	req := httptest.NewRequest("POST", "/callback", strings.NewReader(body))
	req.Header.Set("verif-hash", "wrong-hash")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandlePaymentCallback_FlutterwaveInvalidEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"event":"transfer.completed","data":{"status":"successful"}}`

	bus := &mockBus{}
	h := buildTestHandler(t, bus)
	h.cfg.Env.FlutterwaveVerifHash = "test-flw-hash"

	router := gin.New()
	router.POST("/callback", h.HandlePaymentCallback)

	req := httptest.NewRequest("POST", "/callback", strings.NewReader(body))
	req.Header.Set("verif-hash", "test-flw-hash")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleInitiateCheckout_EmptyBody(t *testing.T) {
	c, w := setupAuthTestContext("rider-123", types.RoleRider)
	c.Request.Method = "POST"
	c.Request.Header.Set("Content-Type", "application/json")

	h := buildTestHandler(t, nil)
	h.HandleInitiateCheckout(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
