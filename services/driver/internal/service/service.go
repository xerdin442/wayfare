package service

import (
	"context"
	"strings"

	repo "github.com/xerdin442/wayfare/services/driver/internal/infra/repository"
	rpc "github.com/xerdin442/wayfare/shared/pkg"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DriverService struct {
	rpc.UnimplementedDriverServiceServer
	repo *repo.DriverRepository
}

func NewDriverService(r *repo.DriverRepository) *DriverService {
	return &DriverService{
		repo: r,
	}
}

func (s *DriverService) GetDriverProfile(ctx context.Context, req *rpc.GetProfileRequest) (*rpc.DriverProfileResponse, error) {
	driver, err := s.repo.GetDriverByID(ctx, req.UserId)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, "Driver not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &rpc.DriverProfileResponse{
		Driver: &rpc.Driver{
			Id:             driver.ID.Hex(),
			Name:           driver.Name,
			ProfilePicture: driver.ProfilePicture,
			CarPlate:       driver.CarPlate,
		},
	}, nil
}

func (s *DriverService) Login(ctx context.Context, req *rpc.LoginRequest) (*rpc.AuthResponse, error) {
	driver, err := s.repo.GetDriverByEmail(ctx, req.Email)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, "Invalid email address")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err := bcrypt.CompareHashAndPassword([]byte(driver.Password), []byte(req.Password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "Incorrect password")
	}

	return &rpc.AuthResponse{
		UserId: driver.ID.Hex(),
	}, nil
}

func (s *DriverService) Signup(ctx context.Context, req *rpc.SignupDriverRequest) (*rpc.AuthResponse, error) {
	_, err := s.repo.GetDriverByEmail(ctx, req.Email)
	if err == nil {
		return nil, status.Error(codes.AlreadyExists, "Driver already exists with this email")
	}

	driver, err := s.repo.CreateDriverAccount(ctx, req)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &rpc.AuthResponse{
		UserId: driver.ID.Hex(),
	}, nil
}
