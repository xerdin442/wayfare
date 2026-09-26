package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/types"
)

const (
	AccessTokenExpiry  = 15 * time.Minute
	RefreshTokenExpiry = 7 * 24 * time.Hour
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type AllClaims struct {
	SubjectID string         `json:"sub"`
	Role      types.UserRole `json:"role"`
	TokenType string         `json:"type"`
	jwt.RegisteredClaims
}

func GenerateTokenPair(userID string, role types.UserRole, secretKey string) (*TokenPair, error) {
	accessToken, err := generateJWT(userID, role, secretKey, AccessTokenExpiry, TokenTypeAccess, "")
	if err != nil {
		return nil, err
	}

	jti, _ := uuid.NewV7()
	refreshToken, err := generateJWT(userID, role, secretKey, RefreshTokenExpiry, TokenTypeRefresh, jti.String())
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func generateJWT(userID string, role types.UserRole, secretKey string, expiry time.Duration, tokenType string, jwtID string) (string, error) {
	now := time.Now()
	claims := AllClaims{
		SubjectID: userID,
		Role:      role,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jwtID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

func parseToken(tokenString, secretKey, expectedType string) (*AllClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&AllClaims{},
		func(token *jwt.Token) (any, error) {
			return []byte(secretKey), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*AllClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	// Prevents a refresh token being used as an access token and vice versa
	if claims.TokenType != expectedType {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}

func ValidateRefreshToken(tokenString string, secretKey string) (*AllClaims, error) {
	return parseToken(tokenString, secretKey, TokenTypeRefresh)
}

func refreshTokenBlacklistKey(jti string) string {
	return "refresh_blacklist:" + jti
}

func RevokeRefreshToken(cache *redis.Client, ctx context.Context, jti string, ttl time.Duration) (bool, error) {
	ttl = max(ttl, time.Second)
	return cache.SetNX(ctx, refreshTokenBlacklistKey(jti), true, ttl).Result()
}

func (m *Middleware) JwtGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			return
		}

		tokenString, found := strings.CutPrefix(authHeader, "Bearer ")
		if !found || tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header"})
			return
		}

		claims, err := parseToken(tokenString, m.cfg.Env.JwtSecret, TokenTypeAccess)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Session expired"})
			return
		}

		n, err := m.cfg.Cache.Exists(c.Request.Context(), "token_blacklist:"+tokenString).Result()
		if err != nil {
			log.Error().Err(err).Msg("Error fetching blacklisted tokens from cache")
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}
		if n > 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Session expired"})
			return
		}

		c.Set("user_id", claims.SubjectID)
		c.Set("user_role", claims.Role)
		c.Set("token_exp", time.Unix(claims.ExpiresAt.Unix(), 0))

		c.Next()
	}
}
