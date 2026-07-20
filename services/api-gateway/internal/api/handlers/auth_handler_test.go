package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/base"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/secrets"
	"github.com/xerdin442/wayfare/shared/types"
	"github.com/xerdin442/wayfare/shared/util"
)

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

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
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
