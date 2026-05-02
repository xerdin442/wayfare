package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/paulmach/orb"
	"github.com/shopspring/decimal"
	"github.com/twpayne/go-polyline"
	repo "github.com/xerdin442/wayfare/services/trip/internal/infra/repository"
	"github.com/xerdin442/wayfare/shared/analytics"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/models"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/tracing"
	"github.com/xerdin442/wayfare/shared/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TripService struct {
	pb.UnimplementedTripServiceServer
	repo       *repo.TripRepository
	queue      messaging.MessageBus
	httpClient *http.Client
}

func NewTripService(r *repo.TripRepository, q messaging.MessageBus) *TripService {
	return &TripService{
		repo:       r,
		queue:      q,
		httpClient: tracing.NewHttpClient(),
	}
}

func (s *TripService) getTripRoute(pickup, destination *pb.Coordinate) (*contracts.OsrmApiResponse, error) {
	url := fmt.Sprintf(
		"http://router.project-osrm.org/route/v1/driving/%f,%f;%f,%f?overview=full&geometries=geojson",
		pickup.Longitude, pickup.Latitude,
		destination.Longitude, destination.Latitude,
	)

	httpResp, err := s.httpClient.Get(url)
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

func (s *TripService) estimateTripFarePerPackage(ctx context.Context, route *pb.Route, pickupCoords orb.Point) (string, []*pb.RideFare, error) {
	// priceConfig := map[repo.CarPackage]{
	// 	repo.PackageSedan:  {BaseFare: 50000, PricePerKm: 15000, PricePerMinute: 2000, MinFare: 150000},
	// 	repo.PackageSUV:    {BaseFare: 100000, PricePerKm: 25000, PricePerMinute: 4000, MinFare: 250000},
	// 	repo.PackageLuxury: {BaseFare: 250000, PricePerKm: 45000, PricePerMinute: 8000, MinFare: 500000},
	// }

	// Get pricing categories per package
	priceConfig, err := s.repo.GetPricingPerRegion(ctx, pickupCoords)
	if err != nil {
		return "", nil, fmt.Errorf("Failed to fetch pricing categories per region: %v", err)
	}

	// Convert OSRM metrics to standard units
	distKm := route.Distance / 1000.0
	durMin := route.Duration / 60.0

	regionID := priceConfig[0].RegionID.Hex()
	rideFares := make([]*pb.RideFare, 0, len(priceConfig))

	// Estimate ride fare per package
	for _, cfg := range priceConfig {
		distanceCost := int64(distKm * float64(cfg.PerKmKobo))
		timeCost := int64(durMin * float64(cfg.PerMinuteKobo))
		totalCost := cfg.BaseFeeKobo + distanceCost + timeCost

		// Apply minimum fare if total cost is below fare threshold
		estimatedPrice := max(totalCost, cfg.MinFareKobo)

		rideFares = append(rideFares, &pb.RideFare{
			PackageSlug:      string(cfg.CarPackage),
			BasePrice:        estimatedPrice / 100,
			TotalPriceInKobo: estimatedPrice,
			Route:            route,
		})
	}

	return regionID, rideFares, nil
}

func (s *TripService) GetTripDetails(ctx context.Context, req *pb.TripDetailsRequest) (*pb.TripDetailsResponse, error) {
	trip, err := s.repo.GetTripByID(ctx, req.TripId)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, "Trip not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.TripDetailsResponse{
		RideFareAmount: trip.Fare.BasePrice,
		UserId:         trip.UserID.Hex(),
		Region:         trip.Region,
	}, nil
}

func (s *TripService) PreviewTrip(ctx context.Context, req *pb.PreviewTripRequest) (*pb.PreviewTripResponse, error) {
	// Extract coordinates
	pickupCoords := orb.Point{req.Pickup.Longitude, req.Pickup.Latitude}
	destinationCoords := orb.Point{req.Destination.Longitude, req.Destination.Latitude}

	// Get trip route
	route, err := s.getTripRoute(req.Pickup, req.Destination)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Estimate ride fares
	regionID, rideFares, err := s.estimateTripFarePerPackage(ctx, route.ToProto(), pickupCoords)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Generate route details
	fullPath := polyline.EncodeCoords(route.Routes[0].Geometry.Coordinates)
	routeDetails := models.RouteDetails{
		Pickup: models.GeoPoint{
			Type:        "Point",
			Coordinates: pickupCoords,
		},
		Destination: models.GeoPoint{
			Type:        "Point",
			Coordinates: destinationCoords,
		},
		Duration: route.ToProto().Duration,
		Distance: route.ToProto().Distance,
		Polyline: string(fullPath),
	}

	// Store generated ride fares
	if err := s.repo.StoreRideFares(ctx, rideFares, routeDetails, req.UserId, regionID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.PreviewTripResponse{
		Route:     route.ToProto(),
		RideFares: rideFares,
	}, nil
}

func (s *TripService) StartTrip(ctx context.Context, req *pb.StartTripRequest) (*pb.StartTripResponse, error) {
	// Create new trip
	trip, err := s.repo.CreateTrip(ctx, req.RideFareId, req.UserId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Find and assign available driver
	payload := messaging.AssignDriverQueuePayload{
		Trip: types.Trip{
			ID:     trip.ID.Hex(),
			UserID: trip.UserID.Hex(),
			Status: trip.Status,
			SelectedFare: types.RideFare{
				ID:               req.RideFareId,
				PackageSlug:      trip.Fare.CarPackage,
				BasePrice:        trip.Fare.BasePrice,
				TotalPriceInKobo: trip.Fare.TotalPriceInKobo,
			},
			Route: types.Route{
				Distance: trip.Route.Distance,
				Duration: trip.Route.Duration,
				Geometry: []*types.Geometry{
					{Coordinates: []*types.Coordinate{
						{
							Longitude: trip.Route.Pickup.Coordinates[0],
							Latitude:  trip.Route.Pickup.Coordinates[1],
						},
						{
							Longitude: trip.Route.Destination.Coordinates[0],
							Latitude:  trip.Route.Destination.Coordinates[1],
						},
					}},
				},
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to marshal assign_driver queue payload")
	}

	msg := messaging.AmqpMessage{Data: data}
	if err := s.queue.PublishMessage(ctx, messaging.ServicesExchange, messaging.TripEventCreated, msg); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to publish %s event", messaging.TripEventCreated)
	}

	tripEvent := &models.TripEventModel{
		TripID:                trip.ID.Hex(),
		Region:                trip.Region,
		CarPackage:            trip.Fare.CarPackage,
		TripStatus:            trip.Status,
		PredictedDurationMins: decimal.NewFromFloat(trip.Route.Duration).Div(decimal.NewFromInt(60)),
		DistanceKm:            decimal.NewFromFloat(trip.Route.Distance).Div(decimal.NewFromInt(1000)),
		PickupLat:             trip.Route.Pickup.Coordinates[1],
		PickupLng:             trip.Route.Pickup.Coordinates[0],
	}
	if err := analytics.SendEvent(ctx, s.queue, tripEvent); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.StartTripResponse{
		TripId: trip.ID.Hex(),
	}, nil
}
