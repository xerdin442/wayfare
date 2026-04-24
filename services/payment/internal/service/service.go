package service

import (
	"context"

	"github.com/redis/go-redis/v9"
	repo "github.com/xerdin442/wayfare/services/payment/internal/infra/repository"
	rpc "github.com/xerdin442/wayfare/shared/pkg"
)

type PaymentService struct {
	rpc.UnimplementedPaymentServiceServer
	repo  *repo.PaymentRepository
	cache *redis.Client
}

func NewPaymentService(r *repo.PaymentRepository, c *redis.Client) *PaymentService {
	return &PaymentService{
		repo:  r,
		cache: c,
	}
}

func (s *PaymentService) InitiatePayment(ctx context.Context, req *rpc.InitiatePaymentRequest) (*rpc.InitiatePaymentResponse, error) {
	return &rpc.InitiatePaymentResponse{}, nil
}
