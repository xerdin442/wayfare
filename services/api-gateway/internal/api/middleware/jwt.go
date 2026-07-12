package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/types"
)

type AllClaims struct {
	SubjectID string         `json:"sub"`
	Role      types.UserRole `json:"role"`
	jwt.RegisteredClaims
}

func GenerateAccessToken(userID string, role types.UserRole, secretKey string) (string, error) {
	claims := AllClaims{
		SubjectID: userID,
		Role:      role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
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

		token, parseErr := jwt.ParseWithClaims(tokenString, &AllClaims{}, func(token *jwt.Token) (any, error) {
			return []byte(m.cfg.Env.JwtSecret), nil
		})

		if parseErr != nil || !token.Valid {
			log.Warn().Err(parseErr).Msg("Failed to parse JWT")
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

		claims, ok := token.Claims.(*AllClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}

		c.Set("user_id", claims.SubjectID)
		c.Set("user_role", claims.Role)
		c.Set("token_exp", time.Unix(claims.ExpiresAt.Unix(), 0))

		c.Next()
	}
}
