package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/base"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/secrets"
	"github.com/xerdin442/wayfare/shared/types"
)

const testSecret = "test-secret-key"

func newTestMiddleware() *Middleware {
	return New(&base.Config{Env: &secrets.Secrets{JwtSecret: testSecret}})
}

func signToken(t *testing.T, method jwt.SigningMethod, claims AllClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return token
}

// runGuard sends a request through JwtGuard and returns the response code.
// Every case here is rejected before the guard reaches the Redis blacklist
// lookup, so no cache is needed.
func runGuard(t *testing.T, authHeader string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/", newTestMiddleware().JwtGuard(), func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func validClaims(tokenType string) AllClaims {
	return AllClaims{
		SubjectID: "user-1",
		Role:      types.RoleRider,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	}
}

func TestJwtGuard_RejectsInvalidTokens(t *testing.T) {
	noExpiry := validClaims(TokenTypeAccess)
	noExpiry.ExpiresAt = nil

	cases := map[string]string{
		"missing header":        "",
		"missing Bearer prefix": signToken(t, jwt.SigningMethodHS256, validClaims(TokenTypeAccess)),
		"empty bearer token":    "Bearer ",
		"refresh token":         "Bearer " + signToken(t, jwt.SigningMethodHS256, validClaims(TokenTypeRefresh)),
		"non-HS256 algorithm":   "Bearer " + signToken(t, jwt.SigningMethodHS512, validClaims(TokenTypeAccess)),
		"no expiry claim":       "Bearer " + signToken(t, jwt.SigningMethodHS256, noExpiry),
	}

	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			if code := runGuard(t, header); code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", code)
			}
		})
	}
}

func TestValidateRefreshToken(t *testing.T) {
	refresh := signToken(t, jwt.SigningMethodHS256, validClaims(TokenTypeRefresh))
	if _, err := ValidateRefreshToken(refresh, testSecret); err != nil {
		t.Fatalf("expected valid refresh token, got %v", err)
	}

	access := signToken(t, jwt.SigningMethodHS256, validClaims(TokenTypeAccess))
	if _, err := ValidateRefreshToken(access, testSecret); err == nil {
		t.Fatal("expected an access token to be rejected as a refresh token")
	}
}

func TestCustomRequestLogger_AttachesContextLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var level zerolog.Level
	r := gin.New()
	r.Use(newTestMiddleware().CustomRequestLogger())
	r.GET("/", func(c *gin.Context) {
		level = log.Ctx(c.Request.Context()).GetLevel()
		c.Status(http.StatusOK)
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if level == zerolog.Disabled {
		t.Fatal("expected handlers to get an enabled logger from the request context")
	}
}
