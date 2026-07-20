package handlers

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/middleware"
	"github.com/xerdin442/wayfare/shared/contracts"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/storage"
	"github.com/xerdin442/wayfare/shared/tracing"
	"github.com/xerdin442/wayfare/shared/types"
	"github.com/xerdin442/wayfare/shared/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *RouteHandler) setRefreshCookie(c *gin.Context, refreshToken string) {
	domain := "localhost"
	if h.cfg.Env.Environment == "production" {
		domain = fmt.Sprintf(".%s", h.cfg.Env.FrontendUrl)
	}

	c.SetCookie(
		"refresh_token",
		refreshToken,
		int(middleware.RefreshTokenExpiry.Seconds()),
		"/api/v1/auth/refresh",
		domain,
		h.cfg.Env.Environment == "production",
		true,
	)
	c.SetSameSite(http.SameSiteStrictMode)
}

func (h *RouteHandler) HandleSignup(c *gin.Context) {
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandleSignup")
	defer span.End()

	logger := log.Ctx(ctx)

	roleHeader := c.GetHeader("X-User-Role")
	role := types.UserRole(roleHeader)
	if role == "" || (role != types.RoleRider && role != types.RoleDriver) {
		tracing.HandleError(span, util.ErrMissingRoleHeader)
		c.JSON(http.StatusBadRequest, gin.H{"message": util.ErrMissingRoleHeader.Error()})
		return
	}

	var userId string

	if role == types.RoleDriver {
		var req contracts.SignupDriverRequest
		if err := c.ShouldBind(&req); err != nil {
			tracing.HandleError(span, err)

			var ve validator.ValidationErrors
			if errors.As(err, &ve) {
				c.JSON(http.StatusBadRequest, gin.H{
					"message": "Validation failed",
					"errors":  util.FormatValidationErrors(err, &req),
				})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
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

			switch err {
			case util.ErrAccountNameMismatch, util.ErrUnsupportedBank:
				c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			case util.ErrGatewayUnavailable, util.ErrApiRequestFailure:
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Driver signup is temporarily unavailable"})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Driver signup failed"})
			}

			return
		}

		profileImage, err := storage.ProcessFileUpload(ctx, h.cfg.Uploader, req.ProfileImage, "/drivers/profile")
		if err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Failed to parse profile image")

			switch err {
			case util.ErrUnsupportedFileType:
				c.JSON(http.StatusUnsupportedMediaType, gin.H{"message": err.Error()})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Driver signup failed"})
			}
			return
		}

		verificationPhotos := make([]string, 0, len(req.VerificationPhotos))
		for _, photo := range req.VerificationPhotos {
			url, err := storage.ProcessFileUpload(ctx, h.cfg.Uploader, photo, "/drivers/verification")
			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to parse verification photo")
				switch err {
				case util.ErrUnsupportedFileType:
					c.JSON(http.StatusUnsupportedMediaType, gin.H{"message": err.Error()})
				default:
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Driver signup failed"})
				}
				return
			}
			verificationPhotos = append(verificationPhotos, url)
		}

		res, err := h.cfg.Clients.Driver.Signup(ctx, &pb.SignupDriverRequest{
			Name:                  req.Name,
			Email:                 req.Email,
			Phone:                 req.Phone,
			CarModel:              req.CarModel,
			CarColor:              req.CarColor,
			CarPlate:              req.CarPlate,
			Password:              req.Password,
			ProfileImage:          profileImage,
			TransferRecipientCode: transferRecipientCode,
			VerificationPhotos:    verificationPhotos,
		})
		if err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Driver signup error")

			st, ok := status.FromError(err)
			if ok {
				switch st.Code() {
				case codes.AlreadyExists:
					c.JSON(http.StatusConflict, gin.H{"message": st.Message()})
				default:
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Driver signup failed"})
				}
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{"error": "Driver signup failed"})
			return
		}

		userId = res.UserId
	}

	if role == types.RoleRider {
		var req contracts.SignupRiderRequest
		if err := c.ShouldBind(&req); err != nil {
			tracing.HandleError(span, err)

			var ve validator.ValidationErrors
			if errors.As(err, &ve) {
				c.JSON(http.StatusBadRequest, gin.H{
					"message": "Validation failed",
					"errors":  util.FormatValidationErrors(err, &req),
				})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
			return
		}

		var profileImage string
		if req.ProfileImage != nil {
			url, err := storage.ProcessFileUpload(ctx, h.cfg.Uploader, req.ProfileImage, "/riders/profile")
			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to parse profile image")

				switch err {
				case util.ErrUnsupportedFileType:
					c.JSON(http.StatusUnsupportedMediaType, gin.H{"message": err.Error()})
				default:
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Rider signup failed"})
				}
				return
			}
			profileImage = url
		} else {
			profileImage = fmt.Sprintf("https://randomuser.me/api/portraits/lego/%d.jpg", rand.Intn(100))
		}

		res, err := h.cfg.Clients.Rider.Signup(ctx, &pb.SignupRiderRequest{
			Name:         req.Name,
			Email:        req.Email,
			Password:     req.Password,
			ProfileImage: profileImage,
		})
		if err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Rider signup error")

			st, ok := status.FromError(err)
			if ok {
				switch st.Code() {
				case codes.AlreadyExists:
					c.JSON(http.StatusConflict, gin.H{"message": st.Message()})
				default:
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Rider signup failed"})
				}
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{"error": "Rider signup failed"})
			return
		}

		userId = res.UserId
	}

	pair, err := middleware.GenerateTokenPair(userId, role, h.cfg.Env.JwtSecret)
	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Failed to generate token pair")
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("%s signup failed", role)})
		return
	}

	h.setRefreshCookie(c, pair.RefreshToken)

	logger.Info().Msg(fmt.Sprintf("%s signup successful", role))
	c.JSON(http.StatusCreated, contracts.APIResponse{
		Data: gin.H{"token": pair.AccessToken},
	})
}

func (h *RouteHandler) HandleLogin(c *gin.Context) {
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandleLogin")
	defer span.End()

	logger := log.Ctx(ctx)

	roleHeader := c.GetHeader("X-User-Role")
	role := types.UserRole(roleHeader)
	if role == "" || (role != types.RoleRider && role != types.RoleDriver) {
		tracing.HandleError(span, util.ErrMissingRoleHeader)
		c.JSON(http.StatusBadRequest, gin.H{"message": util.ErrMissingRoleHeader.Error()})
		return
	}

	var req contracts.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		tracing.HandleError(span, err)

		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Validation failed",
				"errors":  util.FormatValidationErrors(err, &req),
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	var authResp *pb.AuthResponse
	var loginErr error
	if role == types.RoleDriver {
		authResp, loginErr = h.cfg.Clients.Driver.Login(ctx, &pb.LoginRequest{
			Email:    req.Email,
			Password: req.Password,
		})
	} else {
		authResp, loginErr = h.cfg.Clients.Rider.Login(ctx, &pb.LoginRequest{
			Email:    req.Email,
			Password: req.Password,
		})
	}

	if loginErr != nil {
		tracing.HandleError(span, loginErr)
		logger.Error().Err(loginErr).Msg(fmt.Sprintf("%s login error", role))

		st, ok := status.FromError(loginErr)
		if ok {
			switch st.Code() {
			case codes.NotFound, codes.Unauthenticated:
				c.JSON(http.StatusBadRequest, gin.H{"message": st.Message()})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("%s login failed", role)})
			}
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("%s login failed", role)})
		return
	}

	pair, err := middleware.GenerateTokenPair(authResp.UserId, role, h.cfg.Env.JwtSecret)
	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Failed to generate token pair")
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("%s login failed", role)})
		return
	}

	h.setRefreshCookie(c, pair.RefreshToken)

	logger.Info().Msg(fmt.Sprintf("%s login successful", role))
	c.JSON(http.StatusOK, contracts.APIResponse{
		Data: gin.H{"token": pair.AccessToken},
	})
}

func (h *RouteHandler) HandleRefresh(c *gin.Context) {
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandleRefresh")
	defer span.End()

	logger := log.Ctx(ctx)

	roleHeader := c.GetHeader("X-User-Role")
	role := types.UserRole(roleHeader)
	if role == "" || (role != types.RoleRider && role != types.RoleDriver) {
		tracing.HandleError(span, util.ErrMissingRoleHeader)
		c.JSON(http.StatusBadRequest, gin.H{"message": util.ErrMissingRoleHeader.Error()})
		return
	}

	oldToken, err := c.Cookie("refresh_token")
	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("No refresh token found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing refresh token"})
		return
	}

	claims, err := middleware.ValidateRefreshToken(oldToken, h.cfg.Env.JwtSecret)
	if err != nil {
		tracing.HandleError(span, err)
		logger.Warn().Err(err).Msg("Invalid refresh token")

		switch err {
		case jwt.ErrTokenInvalidId, jwt.ErrSignatureInvalid:
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired. Please log in"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Token refresh failed"})
		}
		return
	}

	if claims.Role != role {
		logger.Warn().Msg("Refresh token role does not match X-User-Role header")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired. Please log in"})
		return
	}

	blacklisted, err := middleware.IsRefreshTokenBlacklisted(h.cfg.Cache, ctx, claims.ID)
	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Failed to check refresh token blacklist")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token refresh failed"})
		return
	}
	if blacklisted {
		logger.Warn().Msg("Refresh token has already been used")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired. Please log in"})
		return
	}

	remainingTTL := time.Until(claims.ExpiresAt.Time)
	if err := middleware.BlacklistRefreshToken(h.cfg.Cache, ctx, claims.ID, remainingTTL); err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Failed to blacklist old refresh token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token refresh failed"})
		return
	}

	pair, err := middleware.GenerateTokenPair(claims.SubjectID, claims.Role, h.cfg.Env.JwtSecret)
	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Failed to generate token pair")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token refresh failed"})
		return
	}

	h.setRefreshCookie(c, pair.RefreshToken)

	c.JSON(http.StatusOK, contracts.APIResponse{
		Data: gin.H{"token": pair.AccessToken},
	})
}

func (h *RouteHandler) HandleLogout(c *gin.Context) {
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandleLogout")
	defer span.End()

	logger := log.Ctx(ctx)

	rfToken, err := c.Cookie("refresh_token")
	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("No refresh token found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing refresh token"})
		return
	}

	authHeader := c.GetHeader("Authorization")
	accessToken := strings.TrimPrefix(authHeader, "Bearer ")

	exp, _ := c.Get("token_exp")
	tokenExp := exp.(time.Time)

	if err := h.cfg.Cache.Set(ctx, "token_blacklist:"+accessToken, true, time.Until(tokenExp)).Err(); err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Failed to blacklist access token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Logout error"})
		return
	}

	claims, err := middleware.ValidateRefreshToken(rfToken, h.cfg.Env.JwtSecret)
	if err != nil {
		tracing.HandleError(span, err)
		logger.Warn().Err(err).Msg("Invalid refresh token")
	} else {
		remainingTTL := time.Until(claims.ExpiresAt.Time)
		if err := middleware.BlacklistRefreshToken(h.cfg.Cache, ctx, claims.ID, remainingTTL); err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Failed to blacklist refresh token")
		}
	}

	c.JSON(http.StatusOK, contracts.APIResponse{
		Data: gin.H{"message": "Logged out!"},
	})
}

func (h *RouteHandler) HandleUserProfile(c *gin.Context) {
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

			st, ok := status.FromError(err)
			if ok {
				switch st.Code() {
				case codes.NotFound:
					c.JSON(http.StatusNotFound, gin.H{"message": "Driver account not found"})
				case codes.PermissionDenied:
					c.JSON(http.StatusForbidden, gin.H{"message": st.Message()})
				default:
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch driver profile"})
				}
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch driver profile"})
			return
		}

		c.JSON(http.StatusOK, contracts.APIResponse{
			Data: gin.H{
				"user": types.Driver{
					ID:                  res.Driver.Id,
					Name:                res.Driver.Name,
					Email:               res.Driver.Email,
					Phone:               res.Driver.Phone,
					ProfilePicture:      res.Driver.ProfilePicture,
					CarModel:            res.Driver.CarModel,
					CarColor:            res.Driver.CarColor,
					CarPlate:            res.Driver.CarPlate,
					PackageSlug:         types.CarPackage(res.Driver.PackageSlug),
					CurrentRating:       res.Driver.CurrentRating,
					TotalCompletedTrips: res.Driver.TotalCompletedTrips,
					Tier:                types.DriverTier(res.Driver.Tier),
				},
			},
		})
	}

	if userRole == types.RoleRider {
		res, err := h.cfg.Clients.Rider.GetRiderProfile(ctx, &pb.GetProfileRequest{
			UserId: userID,
		})
		if err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Failed to fetch rider profile")

			st, ok := status.FromError(err)
			if ok {
				switch st.Code() {
				case codes.NotFound:
					c.JSON(http.StatusNotFound, gin.H{"message": "Rider account not found"})
				default:
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch rider profile"})
				}
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch rider profile"})
			return
		}

		c.JSON(http.StatusOK, contracts.APIResponse{
			Data: gin.H{
				"user": types.Rider{
					ID:             res.Rider.Id,
					Email:          res.Rider.Email,
					Name:           res.Rider.Name,
					Phone:          res.Rider.Phone,
					ProfilePicture: res.Rider.ProfilePicture,
				},
			},
		})
	}
}
