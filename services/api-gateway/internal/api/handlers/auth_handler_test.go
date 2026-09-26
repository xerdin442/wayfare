package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/base"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/client"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/secrets"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/types"
	"github.com/xerdin442/wayfare/shared/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockRiderClient stubs Login only. Calling any other method panics via the
// nil embedded interface.
type mockRiderClient struct {
	pb.RiderServiceClient
	loginErr error
}

func (m *mockRiderClient) Login(ctx context.Context, in *pb.LoginRequest, opts ...grpc.CallOption) (*pb.AuthResponse, error) {
	return nil, m.loginErr
}

func TestHandleLogin_MissingRoleHeader(t *testing.T) {
	c, w := newTestContext("POST", "/auth/login", `{"email":"test@test.com","password":"password123"}`, nil)
	h := buildTestHandler(t, nil)
	h.HandleLogin(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["message"] != util.ErrMissingRoleHeader.Error() {
		t.Fatalf("expected missing role header error")
	}
}

func TestHandleLogin_InvalidRoleHeader(t *testing.T) {
	c, w := newTestContext("POST", "/auth/login", `{"email":"test@test.com","password":"password123"}`, map[string]string{
		"X-User-Role": "admin",
	})
	h := buildTestHandler(t, nil)
	h.HandleLogin(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleLogin_EmptyRoleHeader(t *testing.T) {
	c, w := newTestContext("POST", "/auth/login", `{"email":"test@test.com","password":"password123"}`, map[string]string{
		"X-User-Role": "",
	})
	h := buildTestHandler(t, nil)
	h.HandleLogin(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleLogin_InvalidJSON(t *testing.T) {
	c, w := newTestContext("POST", "/auth/login", `{invalid}`, map[string]string{
		"X-User-Role": "rider",
	})
	h := buildTestHandler(t, nil)
	h.HandleLogin(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleLogin_ValidationError(t *testing.T) {
	c, w := newTestContext("POST", "/auth/login", `{"email":"not-an-email","password":"short"}`, map[string]string{
		"X-User-Role": "rider",
	})
	h := buildTestHandler(t, nil)
	h.HandleLogin(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["message"] != "Validation failed" {
		t.Fatalf("expected Validation failed message, got %v", resp)
	}
	errors, ok := resp["errors"].(map[string]interface{})
	if !ok || len(errors) == 0 {
		t.Fatal("expected field errors in response")
	}
}

func TestHandleSignup_MissingRoleHeader(t *testing.T) {
	c, w := newTestContext("POST", "/auth/signup", `{}`, nil)
	h := buildTestHandler(t, nil)
	h.HandleSignup(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["message"] != util.ErrMissingRoleHeader.Error() {
		t.Fatalf("expected missing role header error")
	}
}

func TestHandleSignup_InvalidRoleHeader(t *testing.T) {
	c, w := newTestContext("POST", "/auth/signup", `{}`, map[string]string{
		"X-User-Role": "admin",
	})
	h := buildTestHandler(t, nil)
	h.HandleSignup(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleSignup_RiderInvalidJSON(t *testing.T) {
	c, w := newTestContext("POST", "/auth/signup", `{invalid}`, map[string]string{
		"X-User-Role": "rider",
	})
	h := buildTestHandler(t, nil)
	h.HandleSignup(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleRefresh_MissingRoleHeader(t *testing.T) {
	c, w := newTestContext("POST", "/auth/refresh", "", nil)
	h := buildTestHandler(t, nil)
	h.HandleRefresh(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleRefresh_InvalidRole(t *testing.T) {
	c, w := newTestContext("POST", "/auth/refresh", "", map[string]string{
		"X-User-Role": "admin",
	})
	h := buildTestHandler(t, nil)
	h.HandleRefresh(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleRefresh_MissingCookie(t *testing.T) {
	c, w := newTestContext("POST", "/auth/refresh", "", map[string]string{
		"X-User-Role": "rider",
	})
	h := buildTestHandler(t, nil)
	h.HandleRefresh(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleRefresh_InvalidToken(t *testing.T) {
	c, w := newTestContext("POST", "/auth/refresh", "", map[string]string{
		"X-User-Role": "rider",
	})
	c.Request.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: "invalid-token-string",
	})

	h := buildTestHandler(t, nil)
	h.HandleRefresh(c)

	// A malformed token is the client's problem, not a server error
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleLogout_MissingCookie(t *testing.T) {
	c, w := setupAuthTestContext("rider-123", types.UserRole("rider"))
	h := buildTestHandler(t, nil)
	h.HandleLogout(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestSetRefreshCookie_Production(t *testing.T) {
	c, w := newTestContext("POST", "/", "", nil)
	h := &RouteHandler{
		cfg: &base.Config{
			Env: &secrets.Secrets{
				Environment: "production",
				FrontendUrl: "wayfare.app",
			},
		},
	}
	h.setRefreshCookie(c, "test-refresh-token")

	cookie := w.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "Secure") {
		t.Fatal("expected Secure flag in production cookie")
	}
	if !strings.Contains(cookie, "wayfare.app") {
		t.Fatalf("expected wayfare.app domain in cookie, got: %s", cookie)
	}
	if !strings.Contains(cookie, "refresh_token") {
		t.Fatal("expected refresh_token in cookie name")
	}
}

func TestSetRefreshCookie_Development(t *testing.T) {
	c, w := newTestContext("POST", "/", "", nil)
	h := &RouteHandler{
		cfg: &base.Config{
			Env: &secrets.Secrets{
				Environment: "development",
				FrontendUrl: "wayfare.app",
			},
		},
	}
	h.setRefreshCookie(c, "test-refresh-token")

	cookie := w.Header().Get("Set-Cookie")
	if strings.Contains(cookie, "Secure") {
		t.Fatal("did not expect Secure flag in development cookie")
	}
	if !strings.Contains(cookie, "localhost") {
		t.Fatalf("expected localhost domain in cookie, got: %s", cookie)
	}
	if !strings.Contains(cookie, "refresh_token") {
		t.Fatal("expected refresh_token in cookie name")
	}
}

func TestSetRefreshCookie_PathAndSameSite(t *testing.T) {
	c, w := newTestContext("POST", "/", "", nil)
	h := buildTestHandler(t, nil)
	h.setRefreshCookie(c, "test-refresh-token")

	cookie := w.Header().Get("Set-Cookie")

	// The path must cover /logout as well as /refresh, or logout never receives the cookie
	if !strings.Contains(cookie, "Path=/api/v1/auth;") {
		t.Fatalf("expected Path=/api/v1/auth, got: %s", cookie)
	}
	if !strings.Contains(cookie, "SameSite=Strict") {
		t.Fatalf("expected SameSite=Strict, got: %s", cookie)
	}
}

func TestClearRefreshCookie(t *testing.T) {
	c, w := newTestContext("POST", "/", "", nil)
	h := buildTestHandler(t, nil)
	h.clearRefreshCookie(c)

	cookie := w.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "refresh_token=;") || !strings.Contains(cookie, "Max-Age=0") {
		t.Fatalf("expected an expired, empty refresh cookie, got: %s", cookie)
	}
}

func TestHandleLogin_UnknownEmailAndWrongPasswordAreIndistinguishable(t *testing.T) {
	loginErrs := []error{
		status.Error(codes.NotFound, "invalid email address"),
		status.Error(codes.Unauthenticated, "incorrect password"),
	}

	var bodies []string
	for _, loginErr := range loginErrs {
		c, w := newTestContext("POST", "/auth/login", `{"email":"test@test.com","password":"password123"}`, map[string]string{
			"X-User-Role": "rider",
		})
		h := buildTestHandler(t, nil)
		h.cfg.Clients = &client.Registry{Rider: &mockRiderClient{loginErr: loginErr}}
		h.HandleLogin(c)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %v, got %d", loginErr, w.Code)
		}
		bodies = append(bodies, w.Body.String())
	}

	if bodies[0] != bodies[1] {
		t.Fatalf("login responses differ, which reveals whether the email exists: %q vs %q", bodies[0], bodies[1])
	}
}
