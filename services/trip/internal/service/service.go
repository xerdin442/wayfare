package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	repo "github.com/xerdin442/wayfare/services/trip/internal/infra/repository"
	rpc "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/types"
)

type TripService struct {
	rpc.UnimplementedTripServiceServer
	repo *repo.TripRepository
}

func NewTripService(r *repo.TripRepository) *TripService {
	return &TripService{
		repo: r,
	}
}

func (s *TripService) GetRoute(ctx context.Context, pickup, destination *types.Coordinate) (*types.OsrmApiResponse, error) {
	url := fmt.Sprintf(
		"http://router.project-osrm.org/route/v1/driving/%f,%f;%f,%f?overview=full&geometries=geojson",
		pickup.Longitude, pickup.Latitude,
		destination.Longitude, destination.Latitude,
	)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch routes from OSRM API: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read OSRM API response body: %v", err)
	}

	var osrmResp types.OsrmApiResponse
	if err := json.Unmarshal(body, &osrmResp); err != nil {
		return nil, fmt.Errorf("Failed to parse OSRM API response: %v", err)
	}

	return &osrmResp, nil
}
