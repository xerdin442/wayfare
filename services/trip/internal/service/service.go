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
	"github.com/xerdin442/wayfare/shared/messaging"
	rpc "github.com/xerdin442/wayfare/shared/pkg"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TripService struct {
	rpc.UnimplementedTripServiceServer
	repo  *repo.TripRepository
	queue messaging.MessageBus
}

func NewTripService(r *repo.TripRepository, q messaging.MessageBus) *TripService {
	return &TripService{
		repo:  r,
		queue: q,
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

func (s *TripService) estimateTripFarePerPackage(ctx context.Context, route *rpc.Route, pickupCoords []float64) ([]*rpc.RideFare, error) {
	// priceConfig := map[repo.CarPackage]{
	// 	repo.PackageSedan:  {BaseFare: 50000, PricePerKm: 15000, PricePerMinute: 2000, MinFare: 150000},
	// 	repo.PackageSUV:    {BaseFare: 100000, PricePerKm: 25000, PricePerMinute: 4000, MinFare: 250000},
	// 	repo.PackageLuxury: {BaseFare: 250000, PricePerKm: 45000, PricePerMinute: 8000, MinFare: 500000},
	// }

	// Get pricing categories per package
	priceConfig, err := s.repo.GetPricingPerRegion(ctx, pickupCoords)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch pricing categories per region: %v", err)
	}

	// Convert OSRM metrics to standard units
	distKm := route.Distance / 1000.0
	durMin := route.Duration / 60.0

	rideFares := make([]*rpc.RideFare, len(priceConfig))

	// Estimate ride fare per package
	for _, cfg := range priceConfig {
		distanceCost := int64(distKm * float64(cfg.PerKmKobo))
		timeCost := int64(durMin * float64(cfg.PerMinuteKobo))
		totalCost := cfg.BaseFeeKobo + distanceCost + timeCost

		// Apply minimum fare if total cost is below fare threshold
		estimatedPrice := max(totalCost, cfg.MinFareKobo)

		rideFares = append(rideFares, &rpc.RideFare{
			PackageSlug:      string(cfg.CarPackage),
			BasePrice:        estimatedPrice / 100,
			TotalPriceInKobo: estimatedPrice,
			Route:            route,
		})
	}

	return rideFares, nil
}

func (s *TripService) PreviewTrip(ctx context.Context, req *rpc.PreviewTripRequest) (*rpc.PreviewTripResponse, error) {
	// Extract coordinates
	pickupCoords := []float64{req.Pickup.Longitude, req.Pickup.Latitude}
	destinationCoords := []float64{req.Destination.Longitude, req.Destination.Latitude}

	// Get trip route
	route, err := s.getTripRoute(req.Pickup, req.Destination)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Estimate ride fares
	rideFares, err := s.estimateTripFarePerPackage(ctx, route.ToProto(), pickupCoords)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Generate route details
	fullPath := polyline.EncodeCoords(route.Routes[0].Geometry.Coordinates)
	routeDetails := repo.RouteDetails{
		Pickup: repo.GeoPoint{
			Type:        "Point",
			Coordinates: pickupCoords,
		},
		Destination: repo.GeoPoint{
			Type:        "Point",
			Coordinates: destinationCoords,
		},
		Duration: route.ToProto().Duration,
		Distance: route.ToProto().Distance,
		Polyline: string(fullPath),
	}

	// Store generated ride fares
	if err := s.repo.StoreRideFares(ctx, rideFares, routeDetails, req.UserId); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &rpc.PreviewTripResponse{
		Route:     route.ToProto(),
		RideFares: rideFares,
	}, nil
}

func (s *TripService) StartTrip(ctx context.Context, req *rpc.StartTripRequest) (*rpc.StartTripResponse, error) {
	// Create new trip
	tripID, err := s.repo.CreateTrip(ctx, req.RideFareId, req.UserId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Publish trip created event**

	return &rpc.StartTripResponse{
		TripId: tripID,
	}, nil
}
