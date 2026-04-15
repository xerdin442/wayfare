package handlers

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/middleware"
	"github.com/xerdin442/wayfare/shared/contracts"
	rpc "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/storage"
	"github.com/xerdin442/wayfare/shared/types"
)

func (h *RouteHandler) HandleSignup(c *gin.Context) {
	logger := log.Ctx(c.Request.Context())

	role := c.GetHeader("X-User-Role")
	if role == "" || (types.UserRole(role) != types.RoleRider && types.UserRole(role) != types.RoleDriver) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid X-User-Role header"})
		return
	}

	if types.UserRole(role) == types.RoleDriver {
		var req contracts.SignupDriverRequest
		if err := c.ShouldBind(&req); err != nil {
			logger.Error().Err(err).Msg("Failed to parse driver signup request")
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		file, err := req.ProfileImage.Open()
		if err != nil {
			logger.Error().Err(err).Msg("Failed to parse profile image")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse profile image"})
			return
		}
		defer file.Close()

		if err := storage.ParseImageMimetype(file); err != nil {
			logger.Error().Err(err).Msg("Unsupported MIME type")
			c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": err.Error()})
			return
		}

		result, err := storage.ProcessFileUpload(file, h.cfg.Uploader)
		if err != nil {
			logger.Error().Err(err).Msg("Cloudinary upload error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload profile image"})
			return
		}

		res, err := h.cfg.Clients.Driver.Signup(c.Request.Context(), &rpc.SignupDriverRequest{
			Name:         req.Name,
			Email:        req.Email,
			Password:     req.Password,
			ProfileImage: result.SecureURL,
			CarPackage:   req.CarPackage,
			CarPlate:     req.CarPlate,
		})
		if err != nil {
			logger.Error().Err(err).Msg("Driver signup error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error occurred during signup"})
			return
		}

		// Generate JWT token
		token, err := middleware.GenerateToken(res.UserId, types.RoleDriver, h.cfg.Env.JwtSecret)
		if err != nil {
			logger.Error().Err(err).Msg("Token generation error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate auth token"})
			return
		}

		logger.Info().Str("email", req.Email).Msg("Driver signup successful")
		c.JSON(http.StatusCreated, contracts.APIResponse{
			Data: gin.H{"token": token},
		})

		return
	}

	if types.UserRole(role) == types.RoleRider {
		var req contracts.SignupRiderRequest
		if err := c.ShouldBind(&req); err != nil {
			logger.Error().Err(err).Msg("Failed to parse rider signup request")
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var profilePicURL string
		if req.ProfileImage != nil {
			file, err := req.ProfileImage.Open()
			if err != nil {
				logger.Error().Err(err).Msg("Failed to parse profile image")
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse profile image"})
				return
			}
			defer file.Close()

			if err := storage.ParseImageMimetype(file); err != nil {
				logger.Error().Err(err).Msg("Unsupported MIME type")
				c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": err.Error()})
				return
			}

			result, err := storage.ProcessFileUpload(file, h.cfg.Uploader)
			if err != nil {
				logger.Error().Err(err).Msg("Cloudinary upload error")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload profile image"})
				return
			}
			profilePicURL = result.SecureURL
		} else {
			profilePicURL = fmt.Sprintf("https://randomuser.me/api/portraits/lego/%d.jpg", rand.Intn(100))
		}

		res, err := h.cfg.Clients.Rider.Signup(c.Request.Context(), &rpc.SignupRiderRequest{
			Name:         req.Name,
			Email:        req.Email,
			Password:     req.Password,
			ProfileImage: profilePicURL,
		})
		if err != nil {
			logger.Error().Err(err).Msg("Rider signup error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error occurred during signup"})
			return
		}

		// Generate JWT token
		token, err := middleware.GenerateToken(res.UserId, types.RoleRider, h.cfg.Env.JwtSecret)
		if err != nil {
			logger.Error().Err(err).Msg("Token generation error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate auth token"})
			return
		}

		logger.Info().Str("email", req.Email).Msg("Rider signup successful")
		c.JSON(http.StatusCreated, contracts.APIResponse{
			Data: gin.H{"token": token},
		})

		return
	}
}

func (h *RouteHandler) HandleLogin(c *gin.Context) {
	logger := log.Ctx(c.Request.Context())

	role := c.GetHeader("X-User-Role")
	if role == "" || (types.UserRole(role) != types.RoleRider && types.UserRole(role) != types.RoleDriver) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid X-User-Role header"})
		return
	}

	var req contracts.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error().Err(err).Msg("Failed to parse login request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var authToken string
	if types.UserRole(role) == types.RoleDriver {
		res, err := h.cfg.Clients.Driver.Login(c.Request.Context(), &rpc.LoginRequest{
			Email:    req.Email,
			Password: req.Password,
		})
		if err != nil {
			logger.Error().Err(err).Msg("Driver login error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error occurred during login"})
			return
		}

		// Generate JWT token
		token, err := middleware.GenerateToken(res.UserId, types.RoleDriver, h.cfg.Env.JwtSecret)
		if err != nil {
			logger.Error().Err(err).Msg("Token generation error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate auth token"})
			return
		}

		authToken = token
	} else {
		res, err := h.cfg.Clients.Rider.Login(c.Request.Context(), &rpc.LoginRequest{
			Email:    req.Email,
			Password: req.Password,
		})
		if err != nil {
			logger.Error().Err(err).Msg("Rider login error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error occurred during login"})
			return
		}

		// Generate JWT token
		token, err := middleware.GenerateToken(res.UserId, types.RoleRider, h.cfg.Env.JwtSecret)
		if err != nil {
			logger.Error().Err(err).Msg("Token generation error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate auth token"})
			return
		}

		authToken = token
	}

	logger.Info().Str("email", req.Email).Msg("Login successful")
	c.JSON(http.StatusOK, contracts.APIResponse{
		Data: gin.H{"token": authToken},
	})
}

func (h *RouteHandler) HandleLogout(c *gin.Context) {
	logger := log.Ctx(c.Request.Context())

	authHeader := c.GetHeader("Authorization")
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	exp, _ := c.Get("token_exp")
	tokenExp := exp.(time.Time)

	err := h.cfg.Cache.Set(c.Request.Context(), tokenString, "blacklisted", time.Until(tokenExp)).Err()
	if err != nil {
		logger.Error().Err(err).Msg("Logout error")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token blacklist error"})
		return
	}

	logger.Info().Msg("Logout successful")
	c.JSON(http.StatusOK, contracts.APIResponse{
		Data: gin.H{"message": "Logged out!"},
	})
}

func (h *RouteHandler) HandleUserProfile(c *gin.Context) {
	logger := log.Ctx(c.Request.Context())

	role := c.GetHeader("X-User-Role")
	if role == "" || (types.UserRole(role) != types.RoleRider && types.UserRole(role) != types.RoleDriver) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid X-User-Role header"})
		return
	}

	userID := c.MustGet("user_id").(string)

	if types.UserRole(role) == types.RoleDriver {
		res, err := h.cfg.Clients.Driver.GetDriverByID(c.Request.Context(), &rpc.GetDriverRequest{
			DriverId: userID,
		})
		if err != nil {
			logger.Error().Err(err).Msg("Failed to fetch driver profile")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user profile"})
			return
		}

		c.JSON(http.StatusOK, contracts.APIResponse{
			Data: gin.H{"user": res.Driver},
		})
		return
	}

	if types.UserRole(role) == types.RoleRider {
		res, err := h.cfg.Clients.Rider.GetRiderByID(c.Request.Context(), &rpc.GetRiderRequest{
			RiderId: userID,
		})
		if err != nil {
			logger.Error().Err(err).Msg("Failed to fetch rider profile")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user profile"})
			return
		}

		c.JSON(http.StatusOK, contracts.APIResponse{
			Data: gin.H{"user": res.Rider},
		})
		return
	}
}
