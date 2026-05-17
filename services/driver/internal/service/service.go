package service

import (
	"context"

	repo "github.com/xerdin442/wayfare/services/driver/internal/infra/repository"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/types"
	"github.com/xerdin442/wayfare/shared/util"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DriverService struct {
	pb.UnimplementedDriverServiceServer
	repo *repo.DriverRepository
}

func NewDriverService(r *repo.DriverRepository) *DriverService {
	return &DriverService{
		repo: r,
	}
}

func (s *DriverService) GetDriverProfile(ctx context.Context, req *pb.GetProfileRequest) (*pb.DriverProfileResponse, error) {
	driver, err := s.repo.GetDriverByID(ctx, req.UserId)
	if err != nil {
		if err == util.ErrDocumentNotFound {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}

	// Check driver verification status
	if !driver.IsVerified || driver.CarPackage == types.PackageDefault {
		return nil, status.Error(codes.PermissionDenied, "Your account is not verified yet. Please contact support for assistance")
	}

	// Check if driver has outstanding returns
	if driver.OutstandingReturns > 0 {
		return nil, status.Error(codes.PermissionDenied, "Please, clear your outstanding returns to continue using Wayfare")
	}

	return &pb.DriverProfileResponse{
		Driver: &pb.Driver{
			Id:                  driver.ID.Hex(),
			Name:                driver.Name,
			Email:               driver.Email,
			Phone:               driver.Phone,
			ProfilePicture:      driver.ProfilePicture,
			CarPlate:            driver.CarPlate,
			CarModel:            driver.CarModel,
			CarColor:            driver.CarColor,
			PackageSlug:         string(driver.CarPackage),
			CurrentRating:       driver.CurrentRating,
			TotalCompletedTrips: driver.TotalCompletedTrips,
			Tier:                string(driver.Tier),
		},
	}, nil
}

func (s *DriverService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.AuthResponse, error) {
	existingDriver, err := s.repo.GetDriverByEmail(ctx, req.Email)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal server error")
	}

	if existingDriver == nil {
		return nil, status.Error(codes.NotFound, "invalid email address")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(existingDriver.Password), []byte(req.Password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "Incorrect password")
	}

	return &pb.AuthResponse{
		UserId: existingDriver.ID.Hex(),
	}, nil
}

func (s *DriverService) Signup(ctx context.Context, req *pb.SignupDriverRequest) (*pb.AuthResponse, error) {
	existingDriver, err := s.repo.GetDriverByEmail(ctx, req.Email)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal server error")
	}

	if existingDriver != nil {
		return nil, status.Error(codes.AlreadyExists, "Driver already exists with this email")
	}

	driver, err := s.repo.CreateDriverAccount(ctx, req)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal server error")
	}

	return &pb.AuthResponse{
		UserId: driver.ID.Hex(),
	}, nil
}
