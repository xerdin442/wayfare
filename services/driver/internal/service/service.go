package service

import (
	"context"

	repo "github.com/xerdin442/wayfare/services/driver/internal/infra/repository"
	"github.com/xerdin442/wayfare/shared/messaging"
	rpc "github.com/xerdin442/wayfare/shared/pkg"
)

type DriverService struct {
	rpc.UnimplementedDriverServiceServer
	repo  *repo.DriverRepository
	queue messaging.MessageBus
}

func NewDriverService(r *repo.DriverRepository, q messaging.MessageBus) *DriverService {
	return &DriverService{
		repo:  r,
		queue: q,
	}
}

func (s *DriverService) GetDriverProfile(ctx context.Context, req *rpc.GetProfileRequest) (*rpc.DriverProfileResponse, error) {
	return &rpc.DriverProfileResponse{}, nil
}

func (s *DriverService) Login(ctx context.Context, req *rpc.LoginRequest) (*rpc.AuthResponse, error) {
	return &rpc.AuthResponse{}, nil
}

func (s *DriverService) Signup(ctx context.Context, req *rpc.SignupDriverRequest) (*rpc.AuthResponse, error) {
	return &rpc.AuthResponse{}, nil
}
