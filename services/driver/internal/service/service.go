package service

import (
	"context"

	repo "github.com/xerdin442/wayfare/services/driver/internal/infra/repository"
	rpc "github.com/xerdin442/wayfare/shared/pkg"
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

func (s *DriverService) GetDriverByID(context.Context, *rpc.GetDriverRequest) (*rpc.GetDriverResponse, error)
