package handlers

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/middleware"
	"github.com/xerdin442/wayfare/shared/contracts"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/storage"
	"github.com/xerdin442/wayfare/shared/tracing"
	"github.com/xerdin442/wayfare/shared/types"
)

var (
	ErrMissingRoleHeader = errors.New("missing or invalid X-User-Role header")
)

func (h *RouteHandler) HandleSignup(c *gin.Context) {
	// Start tracer
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandleSignup")
	defer span.End()

	logger := log.Ctx(ctx)

	role := c.GetHeader("X-User-Role")
	if role == "" || (types.UserRole(role) != types.RoleRider && types.UserRole(role) != types.RoleDriver) {
		tracing.HandleError(span, ErrMissingRoleHeader)
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrMissingRoleHeader.Error()})
		return
	}

	if types.UserRole(role) == types.RoleDriver {
		var req contracts.SignupDriverRequest
		if err := c.ShouldBind(&req); err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Failed to parse driver signup request")
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		transferRecipientCode, err := h.createTransferRecipient(ctx, req.Name, &contracts.AccountDetails{
			AccountNumber: req.AccountNumber,
			AccountName:   req.AccountName,
			BankName:      req.BankName,
		})
		if err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Failed to create transfer recipient")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		file, err := req.ProfileImage.Open()
		if err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Failed to parse profile image")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse profile image"})
			return
		}
		defer file.Close()

		if err := storage.ParseImageMimetype(file); err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Unsupported MIME type")
			c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": err.Error()})
			return
		}

		result, err := storage.ProcessFileUpload(ctx, file, h.cfg.Uploader)
		if err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Cloudinary upload error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload profile image"})
			return
		}

		res, err := h.cfg.Clients.Driver.Signup(ctx, &pb.SignupDriverRequest{
			Name:                  req.Name,
			Email:                 req.Email,
			Password:              req.Password,
			ProfileImage:          result.SecureURL,
			CarPackage:            req.CarPackage,
			CarPlate:              req.CarPlate,
			TransferRecipientCode: transferRecipientCode,
		})
		if err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Driver signup error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error occurred during signup"})
			return
		}

		// Generate JWT token
		token, err := middleware.GenerateToken(res.UserId, types.RoleDriver, h.cfg.Env.JwtSecret)
		if err != nil {
			tracing.HandleError(span, err)
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

			result, err := storage.ProcessFileUpload(ctx, file, h.cfg.Uploader)
			if err != nil {
				logger.Error().Err(err).Msg("Cloudinary upload error")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload profile image"})
				return
			}
			profilePicURL = result.SecureURL
		} else {
			profilePicURL = fmt.Sprintf("https://randomuser.me/api/portraits/lego/%d.jpg", rand.Intn(100))
		}

		res, err := h.cfg.Clients.Rider.Signup(ctx, &pb.SignupRiderRequest{
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
	// Start tracer
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandleLogin")
	defer span.End()

	logger := log.Ctx(ctx)

	role := c.GetHeader("X-User-Role")
	if role == "" || (types.UserRole(role) != types.RoleRider && types.UserRole(role) != types.RoleDriver) {
		tracing.HandleError(span, ErrMissingRoleHeader)
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrMissingRoleHeader.Error()})
		return
	}

	var req contracts.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Failed to parse login request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var authToken string
	if types.UserRole(role) == types.RoleDriver {
		res, err := h.cfg.Clients.Driver.Login(ctx, &pb.LoginRequest{
			Email:    req.Email,
			Password: req.Password,
		})
		if err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Driver login error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error occurred during login"})
			return
		}

		// Generate JWT token
		token, err := middleware.GenerateToken(res.UserId, types.RoleDriver, h.cfg.Env.JwtSecret)
		if err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Token generation error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate auth token"})
			return
		}

		authToken = token
	} else {
		res, err := h.cfg.Clients.Rider.Login(ctx, &pb.LoginRequest{
			Email:    req.Email,
			Password: req.Password,
		})
		if err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Rider login error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error occurred during login"})
			return
		}

		// Generate JWT token
		token, err := middleware.GenerateToken(res.UserId, types.RoleRider, h.cfg.Env.JwtSecret)
		if err != nil {
			tracing.HandleError(span, err)
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
	// Start tracer
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandleLogout")
	defer span.End()

	logger := log.Ctx(ctx)

	authHeader := c.GetHeader("Authorization")
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	exp, _ := c.Get("token_exp")
	tokenExp := exp.(time.Time)

	err := h.cfg.Cache.Set(ctx, tokenString, "blacklisted", time.Until(tokenExp)).Err()
	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Logout error")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token blacklist error"})
		return
	}

	c.JSON(http.StatusOK, contracts.APIResponse{
		Data: gin.H{"message": "Logged out!"},
	})
}

func (h *RouteHandler) HandleUserProfile(c *gin.Context) {
	// Start tracer
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandleUserProfile")
	defer span.End()

	logger := log.Ctx(ctx)

	userID := c.MustGet("user_id").(string)
	userRole := c.MustGet("user_role").(types.UserRole)

	if userRole == types.RoleDriver {
		res, err := h.cfg.Clients.Driver.GetDriverProfile(ctx, &pb.GetProfileRequest{
			UserId: userID,
		})
		if err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Failed to fetch driver profile")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user profile"})
			return
		}

		c.JSON(http.StatusOK, contracts.APIResponse{
			Data: gin.H{"user": res.Driver},
		})
		return
	}

	if userRole == types.RoleRider {
		res, err := h.cfg.Clients.Rider.GetRiderProfile(ctx, &pb.GetProfileRequest{
			UserId: userID,
		})
		if err != nil {
			tracing.HandleError(span, err)
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
