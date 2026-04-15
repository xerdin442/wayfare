package service

import (
	"context"

	repo "github.com/xerdin442/wayfare/services/rider/internal/infra/repository"
	"github.com/xerdin442/wayfare/shared/messaging"
	rpc "github.com/xerdin442/wayfare/shared/pkg"
)

type RiderService struct {
	rpc.UnimplementedRiderServiceServer
	repo  *repo.RiderRepository
	queue messaging.MessageBus
}

func NewRiderService(r *repo.RiderRepository, q messaging.MessageBus) *RiderService {
	return &RiderService{
		repo:  r,
		queue: q,
	}
}

func (s *RiderService) GetRiderByID(ctx context.Context, req *rpc.GetRiderRequest) (*rpc.GetRiderResponse, error) {
	return &rpc.GetRiderResponse{}, nil
}

func (s *RiderService) Login(ctx context.Context, req *rpc.LoginRequest) (*rpc.AuthResponse, error) {
	return &rpc.AuthResponse{}, nil
}

func (s *RiderService) Signup(ctx context.Context, req *rpc.SignupRiderRequest) (*rpc.AuthResponse, error) {
	return &rpc.AuthResponse{}, nil
}
