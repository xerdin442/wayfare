package service

import (
	"context"
	"strings"

	repo "github.com/xerdin442/wayfare/services/driver/internal/infra/repository"
	pb "github.com/xerdin442/wayfare/shared/pkg"
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
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, "Driver not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.DriverProfileResponse{
		Driver: &pb.Driver{
			Id:                  driver.ID.Hex(),
			Name:                driver.Name,
			ProfilePicture:      driver.ProfilePicture,
			CarPlate:            driver.CarPlate,
			CurrentRating:       driver.CurrentRating,
			TotalCompletedTrips: driver.TotalCompletedTrips,
			Tier:                string(driver.Tier),
		},
	}, nil
}

func (s *DriverService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.AuthResponse, error) {
	driver, err := s.repo.GetDriverByEmail(ctx, req.Email)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, "Invalid email address")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Check if driver has outstanding returns
	if driver.OutstandingReturns > 0 {
		return nil, status.Error(codes.PermissionDenied, "Please, clear your outstanding returns to continue using Wayfare")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(driver.Password), []byte(req.Password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "Incorrect password")
	}

	return &pb.AuthResponse{
		UserId: driver.ID.Hex(),
	}, nil
}

func (s *DriverService) Signup(ctx context.Context, req *pb.SignupDriverRequest) (*pb.AuthResponse, error) {
	_, err := s.repo.GetDriverByEmail(ctx, req.Email)
	if err == nil {
		return nil, status.Error(codes.AlreadyExists, "Driver already exists with this email")
	}

	driver, err := s.repo.CreateDriverAccount(ctx, req)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.AuthResponse{
		UserId: driver.ID.Hex(),
	}, nil
}
