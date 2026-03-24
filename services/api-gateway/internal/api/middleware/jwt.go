package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
)

type AllClaims struct {
	UserID int32 `json:"user_id"`
	jwt.RegisteredClaims
}

func GenerateToken(userID int32, secretKey string) (string, error) {
	claims := AllClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

func (m *Middleware) JwtGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.ParseWithClaims(tokenString, &AllClaims{}, func(token *jwt.Token) (any, error) {
			return m.cfg.Env.JwtSecret, nil
		})

		// Check if token is blacklisted
		n, err := m.cfg.Cache.Exists(c.Request.Context(), tokenString).Result()
		if err != nil {
			log.Fatal().Err(err).Msg("Error fetching blacklisted tokens from cache")
		}

		if err != nil || !token.Valid || n > 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Session expired. Please log in"})
			return
		}

		claims, ok := token.Claims.(*AllClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("token_exp", time.Unix(claims.ExpiresAt.Unix(), 0))
		c.Next()
	}
}
