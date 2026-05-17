package handlers

import (
	"context"
	"fmt"
	"math/rand"
	"mime/multipart"
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
	"github.com/xerdin442/wayfare/shared/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *RouteHandler) parseAndUploadFile(ctx context.Context, image *multipart.FileHeader, path string) (string, error) {
	file, err := image.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	if err := storage.ParseImageMimetype(file); err != nil {
		return "", err
	}

	result, err := storage.ProcessFileUpload(ctx, file, h.cfg.Uploader, path)
	if err != nil {
		return "", err
	}
	return result.SecureURL, nil
}

func (h *RouteHandler) HandleSignup(c *gin.Context) {
	// Start tracer
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandleSignup")
	defer span.End()

	logger := log.Ctx(ctx)

	role := c.GetHeader("X-User-Role")
	if role == "" || (types.UserRole(role) != types.RoleRider && types.UserRole(role) != types.RoleDriver) {
		tracing.HandleError(span, util.ErrMissingRoleHeader)
		c.JSON(http.StatusBadRequest, gin.H{"message": util.ErrMissingRoleHeader.Error()})
		return
	}

	if types.UserRole(role) == types.RoleDriver {
		var req contracts.SignupDriverRequest
		if err := c.ShouldBind(&req); err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Failed to parse driver signup request")
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
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

		profileImage, err := h.parseAndUploadFile(ctx, req.ProfileImage, "/drivers/profile")
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

		verificationPhotos := make([]string, len(req.VerificationPhotos))
		for i, photo := range req.VerificationPhotos {
			url, err := h.parseAndUploadFile(ctx, photo, "/drivers/verification")
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
			verificationPhotos[i] = url
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

		// Generate JWT token
		token, err := middleware.GenerateToken(res.UserId, types.RoleDriver, h.cfg.Env.JwtSecret)
		if err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Token generation error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Driver signup failed"})
			return
		}

		logger.Info().Msg("Driver signup successful. Awaiting admin verification...")
		c.JSON(http.StatusCreated, contracts.APIResponse{
			Data: gin.H{"token": token},
		})

		return
	}

	if types.UserRole(role) == types.RoleRider {
		var req contracts.SignupRiderRequest
		if err := c.ShouldBind(&req); err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Failed to parse rider signup request")
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}

		var profileImage string
		if req.ProfileImage != nil {
			url, err := h.parseAndUploadFile(ctx, req.ProfileImage, "/drivers/profile")
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

		// Generate JWT token
		token, err := middleware.GenerateToken(res.UserId, types.RoleRider, h.cfg.Env.JwtSecret)
		if err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Token generation error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Rider signup failed"})
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
		tracing.HandleError(span, util.ErrMissingRoleHeader)
		c.JSON(http.StatusBadRequest, gin.H{"message": util.ErrMissingRoleHeader.Error()})
		return
	}

	var req contracts.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Failed to parse login request")
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
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

			st, ok := status.FromError(err)
			if ok {
				switch st.Code() {
				case codes.NotFound, codes.Unauthenticated:
					c.JSON(http.StatusBadRequest, gin.H{"message": st.Message()})
				default:
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Driver login failed"})
				}
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{"error": "Driver login failed"})
			return
		}

		// Generate JWT token
		token, err := middleware.GenerateToken(res.UserId, types.RoleDriver, h.cfg.Env.JwtSecret)
		if err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Token generation error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Driver login failed"})
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

			st, ok := status.FromError(err)
			if ok {
				switch st.Code() {
				case codes.NotFound, codes.Unauthenticated:
					c.JSON(http.StatusBadRequest, gin.H{"message": st.Message()})
				default:
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Rider login failed"})
				}

				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{"error": "Rider login failed"})
			return
		}

		// Generate JWT token
		token, err := middleware.GenerateToken(res.UserId, types.RoleRider, h.cfg.Env.JwtSecret)
		if err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Token generation error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Rider login failed"})
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
		return
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
		return
	}
}
