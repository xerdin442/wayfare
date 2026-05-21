package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/paulmach/orb"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
	"github.com/twpayne/go-polyline"
	repo "github.com/xerdin442/wayfare/services/trip/internal/infra/repository"
	"github.com/xerdin442/wayfare/services/trip/internal/secrets"
	"github.com/xerdin442/wayfare/shared/analytics"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/models"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/tracing"
	"github.com/xerdin442/wayfare/shared/types"
	"github.com/xerdin442/wayfare/shared/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TripService struct {
	pb.UnimplementedTripServiceServer
	repo       *repo.TripRepository
	queue      messaging.MessageBus
	cache      *redis.Client
	env        *secrets.Secrets
	httpClient *http.Client
}

func NewTripService(r *repo.TripRepository, q messaging.MessageBus, c *redis.Client, s *secrets.Secrets) *TripService {
	return &TripService{
		repo:       r,
		queue:      q,
		cache:      c,
		env:        s,
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
		log.Error().Err(err).
			Float64("pickup_lng", pickup.Longitude).
			Float64("pickup_lat", pickup.Latitude).
			Float64("destination_lng", destination.Longitude).
			Float64("destination_lat", destination.Latitude).
			Msg("failed to fetch trip routes from osrm api")

		return nil, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		log.Error().Int("status_code", httpResp.StatusCode).Msg("osrm api error")
		return nil, fmt.Errorf("error fetching routes for trip coordinates")
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		log.Error().Err(err).Msg("failed to read osrm api response body")
		return nil, err
	}

	var osrmResp contracts.OsrmApiResponse
	if err := json.Unmarshal(body, &osrmResp); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal osrm api response")
		return nil, err
	}

	return &osrmResp, nil
}

func (s *TripService) checkWeatherConditions(pickupCoords orb.Point) (float64, error) {
	url := fmt.Sprintf(
		"https://api.openweathermap.org/data/2.5/weather?lat=%f&lon=%f&appid=%s",
		pickupCoords.Lat(), pickupCoords.Lon(), s.env.OpenweatherApiKey,
	)

	httpResp, err := s.httpClient.Get(url)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch weather conditions from openweather api")
		return 0, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		log.Error().Int("status_code", httpResp.StatusCode).Msg("openweather api error")
		return 0, fmt.Errorf("error fetching weather conditions for pickup coordinates")
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		log.Error().Err(err).Msg("failed to read openweather api response body")
		return 0, err
	}

	var openweatherResp contracts.OpenweatherApiResponse
	if err := json.Unmarshal(body, &openweatherResp); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal openweather api response")
		return 0, err
	}

	weatherId := int(openweatherResp.Weather[0].ID)
	var surgeFactor float64 = 1.0

	switch {
	case weatherId >= 200 && weatherId <= 232: // Thunderstorm
		surgeFactor = 1.8
	case weatherId >= 300 && weatherId <= 321: // Drizzle
		surgeFactor = 1.2
	case weatherId >= 500 && weatherId <= 531: // Rain
		surgeFactor = 1.5
	case weatherId >= 800:
		surgeFactor = 1.0
	}

	return surgeFactor, nil
}

func (s *TripService) checkDemandAndSupply(ctx context.Context, pickupCoords orb.Point) (float64, error) {
	// Find available drivers within 5km of the trip request
	availableDrivers, err := s.cache.GeoSearch(ctx, "drivers_locations", &redis.GeoSearchQuery{
		Longitude:  pickupCoords.Lon(),
		Latitude:   pickupCoords.Lat(),
		Radius:     5,
		RadiusUnit: "km",
		Sort:       "ASC",
	}).Result()
	if err != nil {
		return 0, err
	}

	tripRequests, err := s.repo.GetActiveTripRequests(ctx, pickupCoords)
	if err != nil {
		return 0, err
	}

	ratio := float64(tripRequests) / float64(len(availableDrivers))
	switch {
	case ratio >= 5:
		return 1.8, nil
	case ratio >= 3:
		return 1.5, nil
	case ratio >= 1.5:
		return 1.2, nil
	default:
		return 1.0, nil
	}
}

func (s *TripService) estimateRideFares(ctx context.Context, route *pb.Route, pickupCoords orb.Point) (string, []*pb.RideFare, error) {
	// priceConfig := map[repo.CarPackage]{
	// 	repo.PackageSedan:  {BaseFare: 50000, PricePerKm: 15000, PricePerMinute: 2000, MinFare: 150000},
	// 	repo.PackageSUV:    {BaseFare: 100000, PricePerKm: 25000, PricePerMinute: 4000, MinFare: 250000},
	// 	repo.PackageLuxury: {BaseFare: 250000, PricePerKm: 45000, PricePerMinute: 8000, MinFare: 500000},
	// }

	// Get pricing categories per package
	priceConfig, err := s.repo.GetPricingPerRegion(ctx, pickupCoords)
	if err != nil {
		return "", nil, err
	}

	// Get multiplier based on weather conditions
	weatherSurgeFactor, err := s.checkWeatherConditions(pickupCoords)
	if err != nil {
		return "", nil, err
	}

	// Get multiplier based on demand and supply
	demandSupplySurgeFactor, err := s.checkDemandAndSupply(ctx, pickupCoords)
	if err != nil {
		return "", nil, err
	}

	// Convert metric to standard units
	distKm := route.Distance / 1000.0
	durMin := route.Duration / 60.0

	regionID := priceConfig[0].RegionID.Hex()
	rideFares := make([]*pb.RideFare, 0, len(priceConfig))

	// Estimate ride fare per package
	for _, cfg := range priceConfig {
		distanceCost := int64(distKm * float64(cfg.PerKm))
		timeCost := int64(durMin * float64(cfg.PerMinute))
		totalCost := cfg.BaseFee + distanceCost + timeCost

		// Apply minimum fare if total cost is below fare threshold
		estimatedPrice := max(totalCost, cfg.MinFare)

		// Apply surge factors
		surgeFactor := max(2.4, weatherSurgeFactor*demandSupplySurgeFactor)
		estimatedPrice = int64(float64(estimatedPrice) * surgeFactor)

		// Check if trip is eligible for after hours fee
		location, _ := time.LoadLocation("Africa/Lagos")
		hour := time.Now().In(location).Hour()

		if hour >= 0 && hour < 5 {
			estimatedPrice += cfg.AfterHoursFee
		}

		// Round up to nearest hundred
		rideAmount := ((estimatedPrice + 99) / 100) * 100

		rideFares = append(rideFares, &pb.RideFare{
			PackageSlug: string(cfg.CarPackage),
			Amount:      rideAmount,
			Route:       route,
		})
	}

	return regionID, rideFares, nil
}

func (s *TripService) GetTripDetails(ctx context.Context, req *pb.TripDetailsRequest) (*pb.TripDetailsResponse, error) {
	trip, err := s.repo.GetTripByID(ctx, req.TripId)
	if err != nil {
		if err == util.ErrDocumentNotFound {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}

	return &pb.TripDetailsResponse{
		RideFare: trip.RideFare,
		UserId:   trip.UserID.Hex(),
		Region:   trip.Region,
	}, nil
}

func (s *TripService) PreviewTrip(ctx context.Context, req *pb.PreviewTripRequest) (*pb.PreviewTripResponse, error) {
	// Check if the user rated their last trip
	lastTrip, err := s.repo.GetLastUnratedTrip(ctx, req.UserId)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal server error")
	}

	if lastTrip != nil {
		data, err := json.Marshal(contracts.WebsocketMessage{
			Type: string(messaging.TripEventRatingRequired),
			Data: contracts.TripRatingRequiredResponse{
				TripId: lastTrip.ID.Hex(),
				Pickup: types.Coordinate{
					Latitude:  lastTrip.Route.Pickup.Coordinates.Lat(),
					Longitude: lastTrip.Route.Pickup.Coordinates.Lon(),
				},
				Destination: types.Coordinate{
					Latitude:  lastTrip.Route.Destination.Coordinates.Lat(),
					Longitude: lastTrip.Route.Destination.Coordinates.Lon(),
				},
				Date: lastTrip.CreatedAt,
			},
		})
		if err != nil {
			return nil, status.Error(codes.Internal, "internal server error")
		}

		if err := s.queue.PublishMessage(
			ctx,
			messaging.GatewayExchange,
			messaging.AmqpEvent(fmt.Sprintf("user.%s", req.UserId)),
			messaging.AmqpMessage{Data: data},
		); err != nil {
			return nil, status.Error(codes.Internal, "internal server error")
		}

		return nil, status.Error(codes.FailedPrecondition, util.ErrTripRatingRequired.Error())
	}

	// Extract coordinates
	pickupCoords := orb.Point{req.Pickup.Longitude, req.Pickup.Latitude}
	destinationCoords := orb.Point{req.Destination.Longitude, req.Destination.Latitude}

	// Get trip route
	route, err := s.getTripRoute(req.Pickup, req.Destination)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal server error")
	}

	// Estimate ride fares
	regionId, rideFares, err := s.estimateRideFares(ctx, route.ToProto(), pickupCoords)
	if err != nil {
		if err == util.ErrUnsupportedLocation {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}

	var polylineString string
	if len(route.Routes) > 0 {
		polylineString = string(polyline.EncodeCoords(route.Routes[0].Geometry.Coordinates))
	}

	// Generate route details
	routeDetails := models.RouteDetails{
		Pickup: models.GeoPoint{
			Type:        "Point",
			Coordinates: pickupCoords,
		},
		Destination: models.GeoPoint{
			Type:        "Point",
			Coordinates: destinationCoords,
		},
		Addresses: []string{req.Pickup.Address, req.Destination.Address},
		Duration:  route.ToProto().Duration,
		Distance:  route.ToProto().Distance,
		Polyline:  polylineString,
	}

	// Store generated ride fares
	if err := s.repo.StoreRideFares(ctx, rideFares, routeDetails, req.UserId, regionId); err != nil {
		return nil, status.Error(codes.Internal, "internal server error")
	}

	return &pb.PreviewTripResponse{
		RideFares: rideFares,
	}, nil
}

func (s *TripService) StartTrip(ctx context.Context, req *pb.StartTripRequest) (*pb.StartTripResponse, error) {
	// Create new trip
	trip, err := s.repo.CreateTrip(ctx, req.RideFareId, req.UserId)
	if err != nil {
		if err == util.ErrTripSessionExpired {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}

	// Find and assign available driver
	payload := messaging.AssignDriverQueuePayload{
		Trip: types.Trip{
			ID:     trip.ID.Hex(),
			UserID: trip.UserID.Hex(),
			Status: trip.Status,
			SelectedFare: types.RideFare{
				ID:          req.RideFareId,
				PackageSlug: trip.CarPackage,
				Amount:      trip.RideFare,
				Route: types.Route{
					Distance: trip.Route.Distance,
					Duration: trip.Route.Duration,
					Geometry: []*types.Geometry{
						{
							Coordinates: []*types.Coordinate{
								{
									Latitude:  trip.Route.Pickup.Coordinates.Lat(),
									Longitude: trip.Route.Pickup.Coordinates.Lon(),
								},
								{
									Latitude:  trip.Route.Destination.Coordinates.Lat(),
									Longitude: trip.Route.Destination.Coordinates.Lon(),
								},
							},
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal server error")
	}

	msg := messaging.AmqpMessage{Data: data}
	if err := s.queue.PublishMessage(ctx, messaging.ServicesExchange, messaging.TripEventCreated, msg); err != nil {
		return nil, status.Error(codes.Internal, "internal server error")
	}

	tripEvent := &models.TripEventModel{
		TripID:                trip.ID.Hex(),
		Region:                trip.Region,
		CarPackage:            trip.CarPackage,
		TripStatus:            trip.Status,
		PredictedDurationMins: decimal.NewFromFloat(trip.Route.Duration).Div(decimal.NewFromInt(60)),
		DistanceKm:            decimal.NewFromFloat(trip.Route.Distance).Div(decimal.NewFromInt(1000)),
		PickupLat:             trip.Route.Pickup.Coordinates.Lat(),
		PickupLng:             trip.Route.Pickup.Coordinates.Lon(),
	}
	if err := analytics.SendEvent(ctx, s.queue, tripEvent); err != nil {
		return nil, status.Error(codes.Internal, "internal server error")
	}

	return &pb.StartTripResponse{
		TripId: trip.ID.Hex(),
	}, nil
}

func (s *TripService) GetTripHistory(ctx context.Context, req *pb.TripHistoryRequest) (*pb.TripHistoryResponse, error) {
	tripModels, err := s.repo.GetUserTripHistory(ctx, req.UserId)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal server error")
	}

	return &pb.TripHistoryResponse{
		Trips: contracts.MapTripModels(tripModels),
	}, nil
}
