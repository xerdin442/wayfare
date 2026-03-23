package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/twpayne/go-polyline"
	repo "github.com/xerdin442/wayfare/services/trip/internal/infra/repository"
	"github.com/xerdin442/wayfare/shared/contracts"
	rpc "github.com/xerdin442/wayfare/shared/pkg"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func (s *TripService) getTripRoute(pickup, destination *rpc.Coordinate) (*contracts.OsrmApiResponse, error) {
	url := fmt.Sprintf(
		"http://router.project-osrm.org/route/v1/driving/%f,%f;%f,%f?overview=full&geometries=geojson",
		pickup.Longitude, pickup.Latitude,
		destination.Longitude, destination.Latitude,
	)

	httpResp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch routes from OSRM API: %v", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read OSRM API response body: %v", err)
	}

	var osrmResp contracts.OsrmApiResponse
	if err := json.Unmarshal(body, &osrmResp); err != nil {
		return nil, fmt.Errorf("Failed to parse OSRM API response: %v", err)
	}

	return &osrmResp, nil
}

func (s *TripService) PreviewTrip(ctx context.Context, req *rpc.PreviewTripRequest) (*rpc.PreviewTripResponse, error) {
	route, err := s.getTripRoute(req.Pickup, req.Destination)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	tripPath := polyline.EncodeCoords(route.Routes[0].Geometry.Coordinates)

	return &rpc.PreviewTripResponse{
		Route:     route.ToProto(),
		RideFares: []*rpc.RideFare{},
	}, nil
}

func (s *TripService) StartTrip(ctx context.Context, req *rpc.StartTripRequest) (*rpc.StartTripResponse, error)
