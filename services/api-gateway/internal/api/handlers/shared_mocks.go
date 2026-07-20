package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/base"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/secrets"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/storage"
	"github.com/xerdin442/wayfare/shared/types"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace/noop"
)

func init() {
	otel.SetTracerProvider(noop.NewTracerProvider())
}

func computeHMACSHA512(secret, body string) string {
	h := hmac.New(sha512.New, []byte(secret))
	h.Write([]byte(body))
	return hex.EncodeToString(h.Sum(nil))
}

func newTestContext(method, path, body string, headers map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c.Request = req
	return c, w
}

func setupAuthTestContext(userID string, role types.UserRole) (*gin.Context, *httptest.ResponseRecorder) {
	c, w := newTestContext("GET", "/", "", nil)
	c.Set("user_id", userID)
	c.Set("user_role", role)
	c.Set("token_exp", time.Now().Add(15*time.Minute))
	c.Request.Header.Set("Authorization", "Bearer mock-access-token")
	return c, w
}

type mockBus struct {
	mu           sync.Mutex
	publishFn    func(context.Context, messaging.AmqpExchange, messaging.AmqpEvent, messaging.AmqpMessage) error
	publishCalls []publishCall
}

type publishCall struct {
	Exchange   messaging.AmqpExchange
	RoutingKey messaging.AmqpEvent
	Msg        messaging.AmqpMessage
}

func (m *mockBus) PublishMessage(ctx context.Context, exchange messaging.AmqpExchange, routingKey messaging.AmqpEvent, msg messaging.AmqpMessage) error {
	m.mu.Lock()
	m.publishCalls = append(m.publishCalls, publishCall{Exchange: exchange, RoutingKey: routingKey, Msg: msg})
	m.mu.Unlock()
	if m.publishFn != nil {
		return m.publishFn(ctx, exchange, routingKey, msg)
	}
	return nil
}

func (m *mockBus) ConsumeMessages(queueName messaging.AmqpQueue, handler func(context.Context, amqp.Delivery) error) error {
	return nil
}

func buildTestHandler(t *testing.T, bus *mockBus) *RouteHandler {
	t.Helper()
	if bus == nil {
		bus = &mockBus{}
	}
	return &RouteHandler{
		cfg: &base.Config{
			Env: &secrets.Secrets{
				Environment:          "development",
				FrontendUrl:          "wayfare.app",
				JwtSecret:            "test-secret-key",
				PaystackSecretKey:    "test-paystack-secret",
				FlutterwaveVerifHash: "test-flw-hash",
			},
			Queue:    bus,
			Tracer:   otel.Tracer("test"),
			Uploader: &storage.FileUploadConfig{Folder: "test"},
		},
		ws: websocket.Upgrader{},
	}
}

func TestMockBusImplementsInterface(t *testing.T) {
	var _ messaging.MessageBus = (*mockBus)(nil)
}

func TestComputeHMACSHA512(t *testing.T) {
	sig := computeHMACSHA512("secret", "body")
	if sig == "" {
		t.Fatal("expected non-empty HMAC")
	}
	if len(sig) != 128 {
		t.Fatalf("expected 128 hex chars for SHA-512, got %d", len(sig))
	}

	sig2 := computeHMACSHA512("secret", "body")
	if sig != sig2 {
		t.Fatal("HMAC should be deterministic")
	}

	sig3 := computeHMACSHA512("secret", "different")
	if sig == sig3 {
		t.Fatal("different payloads should produce different HMACs")
	}
}
