package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xerdin442/wayfare/shared/messaging"
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
	c.Params = gin.Params{{Key: "id", Value: "trip-1"}}

	h := buildTestHandler(t, nil)
	h.HandleInitiateCheckout(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlePaymentCallback_ValidPaystackWebhookIsPublished(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"event":"charge.success","data":{"reference":"ref-123","status":"success","amount":500000}}`
	sig := computeHMACSHA512("test-paystack-secret", body)

	bus := &mockBus{}
	h := buildTestHandler(t, bus)

	router := gin.New()
	router.POST("/callback", h.HandlePaymentCallback)

	req := httptest.NewRequest("POST", "/callback", strings.NewReader(body))
	req.Header.Set("x-paystack-signature", sig)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	payload := publishedWebhook(t, bus)
	if payload.Provider != types.ProviderPaystack || payload.PaystackWebhook.Data.Reference != "ref-123" {
		t.Fatalf("expected the parsed paystack webhook to be published, got %+v", payload)
	}
}

func TestHandlePaymentCallback_ValidFlutterwaveWebhookIsPublished(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"event":"charge.completed","data":{"status":"successful","tx_ref":"ref-456","amount":5000}}`

	bus := &mockBus{}
	h := buildTestHandler(t, bus)

	router := gin.New()
	router.POST("/callback", h.HandlePaymentCallback)

	req := httptest.NewRequest("POST", "/callback", strings.NewReader(body))
	req.Header.Set("verif-hash", "test-flw-hash")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	payload := publishedWebhook(t, bus)
	if payload.Provider != types.ProviderFlutterwave || payload.FlutterwaveWebhook.Data.TxRef != "ref-456" {
		t.Fatalf("expected the parsed flutterwave webhook to be published, got %+v", payload)
	}
}

func publishedWebhook(t *testing.T, bus *mockBus) messaging.PaymentWebhookPayload {
	t.Helper()

	if len(bus.publishCalls) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(bus.publishCalls))
	}
	call := bus.publishCalls[0]
	if call.RoutingKey != messaging.PaymentEventWebhookReceived {
		t.Fatalf("expected %s, got %s", messaging.PaymentEventWebhookReceived, call.RoutingKey)
	}

	var payload messaging.PaymentWebhookPayload
	if err := json.Unmarshal(call.Msg.Data, &payload); err != nil {
		t.Fatalf("failed to decode published payload: %v", err)
	}
	return payload
}

// newCheckoutContext builds a checkout request. A non-empty tripId targets the ride-fare
// route; an empty one targets the driver returns route.
func newCheckoutContext(role types.UserRole, tripId, body string) (*gin.Context, *httptest.ResponseRecorder) {
	c, w := setupAuthTestContext("user-123", role)
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if tripId != "" {
		c.Params = gin.Params{{Key: "id", Value: tripId}}
	}
	return c, w
}

func TestHandleInitiateCheckout_RejectsWrongRole(t *testing.T) {
	cases := map[string]struct {
		role   types.UserRole
		tripId string
	}{
		"driver paying a ride fare": {types.RoleDriver, "trip-1"},
		"rider paying returns":      {types.RoleRider, ""},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, w := newCheckoutContext(tc.role, tc.tripId, `{"email":"user@test.com","tripRating":5}`)
			h := buildTestHandler(t, nil)
			h.HandleInitiateCheckout(c)

			if w.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestHandleInitiateCheckout_RideFareRequiresRating(t *testing.T) {
	c, w := newCheckoutContext(types.RoleRider, "trip-1", `{"email":"user@test.com"}`)
	h := buildTestHandler(t, nil)
	h.HandleInitiateCheckout(c)

	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "tripRating") {
		t.Fatalf("expected 400 for a missing tripRating, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleInitiateCheckout_RejectsNegativeTip(t *testing.T) {
	c, w := newCheckoutContext(types.RoleRider, "trip-1", `{"email":"user@test.com","tripRating":5,"driverTip":-500}`)
	h := buildTestHandler(t, nil)
	h.HandleInitiateCheckout(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a negative tip, got %d: %s", w.Code, w.Body.String())
	}
}
