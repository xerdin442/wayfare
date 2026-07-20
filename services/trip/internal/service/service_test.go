package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/paulmach/orb"
	"github.com/redis/go-redis/v9"
	"github.com/xerdin442/wayfare/services/trip/internal/secrets"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/models"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/types"
	"github.com/xerdin442/wayfare/shared/util"
	"go.mongodb.org/mongo-driver/v2/bson"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type tripRepoStub struct {
	getLastUnratedTripFn  func(ctx context.Context, userId string) (*models.TripModel, error)
	getPricingPerRegionFn func(ctx context.Context, regionId string) ([]*models.PricingModel, error)
	getActiveRequestsFn   func(ctx context.Context, pickupCoords orb.Point) (int64, error)
	storeRideFaresFn      func(ctx context.Context, rideFares []*pb.RideFare, route models.RouteDetails, userId, regionId string) error
	createTripFn          func(ctx context.Context, fareId, userId string) (*models.TripModel, error)
	getTripByIDFn         func(ctx context.Context, tripId string) (*models.TripModel, error)
	getUserTripHistoryFn  func(ctx context.Context, userId string) ([]*models.TripModel, error)
	getRegionBoundsFn     func(ctx context.Context, pickupCoords orb.Point) (*models.RegionModel, error)
}

func (s *tripRepoStub) GetLastUnratedTrip(ctx context.Context, userId string) (*models.TripModel, error) {
	return s.getLastUnratedTripFn(ctx, userId)
}
func (s *tripRepoStub) GetPricingPerRegion(ctx context.Context, regionId string) ([]*models.PricingModel, error) {
	return s.getPricingPerRegionFn(ctx, regionId)
}
func (s *tripRepoStub) GetActiveTripRequests(ctx context.Context, pickupCoords orb.Point) (int64, error) {
	return s.getActiveRequestsFn(ctx, pickupCoords)
}
func (s *tripRepoStub) StoreRideFares(ctx context.Context, rideFares []*pb.RideFare, route models.RouteDetails, userId, regionId string) error {
	return s.storeRideFaresFn(ctx, rideFares, route, userId, regionId)
}
func (s *tripRepoStub) CreateTrip(ctx context.Context, fareId, userId string) (*models.TripModel, error) {
	return s.createTripFn(ctx, fareId, userId)
}
func (s *tripRepoStub) GetTripByID(ctx context.Context, tripId string) (*models.TripModel, error) {
	return s.getTripByIDFn(ctx, tripId)
}
func (s *tripRepoStub) GetUserTripHistory(ctx context.Context, userId string) ([]*models.TripModel, error) {
	return s.getUserTripHistoryFn(ctx, userId)
}
func (s *tripRepoStub) GetRegionBounds(ctx context.Context, pickupCoords orb.Point) (*models.RegionModel, error) {
	return s.getRegionBoundsFn(ctx, pickupCoords)
}

type cacheStub struct {
	scanFn func(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
}

func (c *cacheStub) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
	return c.scanFn(ctx, cursor, match, count)
}

type mockBus struct {
	publishFn func(ctx context.Context, exchange messaging.AmqpExchange, routingKey messaging.AmqpEvent, msg messaging.AmqpMessage) error
}

func (m *mockBus) PublishMessage(ctx context.Context, exchange messaging.AmqpExchange, routingKey messaging.AmqpEvent, msg messaging.AmqpMessage) error {
	if m.publishFn != nil {
		return m.publishFn(ctx, exchange, routingKey, msg)
	}
	return nil
}

func (m *mockBus) ConsumeMessages(queueName messaging.AmqpQueue, handler func(context.Context, amqp.Delivery) error) error {
	return nil
}

func grpcCode(err error) codes.Code {
	st, ok := status.FromError(err)
	if !ok {
		return codes.OK
	}
	return st.Code()
}

func grpcMessage(err error) string {
	st, ok := status.FromError(err)
	if !ok {
		return ""
	}
	return st.Message()
}

func buildTripService(repo *tripRepoStub, cache *cacheStub, bus *mockBus) *TripService {
	if bus == nil {
		bus = &mockBus{}
	}
	return &TripService{
		repo:       repo,
		queue:      bus,
		cache:      cache,
		env:        &secrets.Secrets{OpenweatherApiKey: "test-key"},
		httpClient: http.DefaultClient,
	}
}

func newTestCache(activeDriverKeys ...string) *cacheStub {
	keys := activeDriverKeys
	return &cacheStub{
		scanFn: func(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
			cmd := redis.NewScanCmd(ctx, nil, "scan", 0)
			cmd.SetVal(keys, 0)
			return cmd
		},
	}
}

func makeUnratedTrip() *models.TripModel {
	id := bson.NewObjectID()
	return &models.TripModel{
		ID:     id,
		UserID: bson.NewObjectID(),
		Route: models.RouteDetails{
			Pickup: models.GeoPoint{
				Coordinates: orb.Point{3.3792, 6.5244},
			},
			Destination: models.GeoPoint{
				Coordinates: orb.Point{3.4064, 6.4654},
			},
			Addresses: []string{"Pickup St", "Destination Ave"},
		},
		CreatedAt: time.Now(),
	}
}

func makePricingModels() []*models.PricingModel {
	return []*models.PricingModel{
		{
			RegionID:      bson.NewObjectID(),
			CarPackage:    types.PackageSedan,
			BaseFee:       40000,
			PerKm:         15000,
			PerMinute:     4000,
			AfterHoursFee: 50000,
			MinFare:       100000,
		},
		{
			RegionID:      bson.NewObjectID(),
			CarPackage:    types.PackageSUV,
			BaseFee:       60000,
			PerKm:         20000,
			PerMinute:     5000,
			AfterHoursFee: 50000,
			MinFare:       150000,
		},
	}
}

func osrmResponseJSON() string {
	return `{
		"routes": [{
			"distance": 12000.0,
			"duration": 1800.0,
			"geometry": {
				"coordinates": [[3.3792, 6.5244], [3.3900, 6.5000], [3.4064, 6.4654]],
				"type": "LineString"
			}
		}]
	}`
}

func weatherResponseJSON(id int) string {
	return fmt.Sprintf(`{"weather":[{"id":%d}],"visibility":10000}`, id)
}

func makeCompletedTrip() *models.TripModel {
	id := bson.NewObjectID()
	return &models.TripModel{
		ID:         id,
		RideFare:   250000,
		UserID:     bson.NewObjectID(),
		Region:     "Lagos",
		Status:     types.TripStatusCompleted,
		CarPackage: types.CarPackage("sedan"),
		Route: models.RouteDetails{
			Pickup: models.GeoPoint{
				Coordinates: orb.Point{3.3792, 6.5244},
			},
			Destination: models.GeoPoint{
				Coordinates: orb.Point{3.4064, 6.4654},
			},
			Addresses: []string{"A", "B"},
			Duration:  1800,
			Distance:  12000,
		},
		CreatedAt: time.Now(),
	}
}

type interceptRoundTripper struct {
	osrmURL    string
	weatherURL string
}

func (rt *interceptRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	if strings.Contains(clone.URL.Host, "osrm") && rt.osrmURL != "" {
		clone.URL.Scheme = "http"
		clone.URL.Host = strings.TrimPrefix(rt.osrmURL, "http://")
	} else if strings.Contains(clone.URL.Host, "openweathermap") && rt.weatherURL != "" {
		clone.URL.Scheme = "http"
		clone.URL.Host = strings.TrimPrefix(rt.weatherURL, "http://")
	}
	return http.DefaultTransport.RoundTrip(clone)
}

func TestTripGetDetails_Success(t *testing.T) {
	trip := makeCompletedTrip()
	svc := buildTripService(&tripRepoStub{
		getTripByIDFn: func(ctx context.Context, tripId string) (*models.TripModel, error) {
			if tripId != trip.ID.Hex() {
				t.Fatalf("unexpected trip id: %s", tripId)
			}
			return trip, nil
		},
	}, nil, nil)

	resp, err := svc.GetTripDetails(context.Background(), &pb.TripDetailsRequest{
		TripId: trip.ID.Hex(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.RideFare != 250000 {
		t.Fatalf("expected ride fare 250000, got %d", resp.RideFare)
	}
	if resp.Region != "Lagos" {
		t.Fatalf("expected Lagos, got %s", resp.Region)
	}
}

func TestTripGetDetails_NotFound(t *testing.T) {
	svc := buildTripService(&tripRepoStub{
		getTripByIDFn: func(ctx context.Context, tripId string) (*models.TripModel, error) {
			return nil, util.ErrDocumentNotFound
		},
	}, nil, nil)

	_, err := svc.GetTripDetails(context.Background(), &pb.TripDetailsRequest{
		TripId: "507f1f77bcf86cd799439011",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", grpcCode(err))
	}
}

func TestTripGetDetails_InternalError(t *testing.T) {
	svc := buildTripService(&tripRepoStub{
		getTripByIDFn: func(ctx context.Context, tripId string) (*models.TripModel, error) {
			return nil, fmt.Errorf("db error")
		},
	}, nil, nil)

	_, err := svc.GetTripDetails(context.Background(), &pb.TripDetailsRequest{
		TripId: "507f1f77bcf86cd799439011",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", grpcCode(err))
	}
}

func TestTripGetHistory_Success(t *testing.T) {
	trip := makeCompletedTrip()
	svc := buildTripService(&tripRepoStub{
		getUserTripHistoryFn: func(ctx context.Context, userId string) ([]*models.TripModel, error) {
			if userId != "user123" {
				t.Fatalf("unexpected user id: %s", userId)
			}
			return []*models.TripModel{trip}, nil
		},
	}, nil, nil)

	resp, err := svc.GetTripHistory(context.Background(), &pb.TripHistoryRequest{
		UserId: "user123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Trips) != 1 {
		t.Fatalf("expected 1 trip, got %d", len(resp.Trips))
	}
}

func TestTripGetHistory_Error(t *testing.T) {
	svc := buildTripService(&tripRepoStub{
		getUserTripHistoryFn: func(ctx context.Context, userId string) ([]*models.TripModel, error) {
			return nil, fmt.Errorf("db error")
		},
	}, nil, nil)

	_, err := svc.GetTripHistory(context.Background(), &pb.TripHistoryRequest{
		UserId: "user123",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", grpcCode(err))
	}
}

func TestTripGetRegionBounds_Success(t *testing.T) {
	regionId := bson.NewObjectID()
	svc := buildTripService(&tripRepoStub{
		getRegionBoundsFn: func(ctx context.Context, pickupCoords orb.Point) (*models.RegionModel, error) {
			return &models.RegionModel{
				ID: regionId,
				Boundary: models.GeoPolygon{
					Coordinates: orb.Polygon{
						orb.Ring{
							orb.Point{3.30, 6.40},
							orb.Point{3.50, 6.40},
							orb.Point{3.50, 6.60},
							orb.Point{3.30, 6.60},
						},
					},
				},
			}, nil
		},
	}, nil, nil)

	resp, err := svc.GetRegionBounds(context.Background(), &pb.RegionBoundsRequest{
		Pickup: &pb.Coordinate{Longitude: 3.3792, Latitude: 6.5244},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.RegionId != regionId.Hex() {
		t.Fatalf("expected region id %s, got %s", regionId.Hex(), resp.RegionId)
	}
}

func TestTripGetRegionBounds_Unavailable(t *testing.T) {
	svc := buildTripService(&tripRepoStub{
		getRegionBoundsFn: func(ctx context.Context, pickupCoords orb.Point) (*models.RegionModel, error) {
			return nil, util.ErrUnsupportedLocation
		},
	}, nil, nil)

	resp, err := svc.GetRegionBounds(context.Background(), &pb.RegionBoundsRequest{
		Pickup: &pb.Coordinate{Longitude: 1.0, Latitude: 2.0},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Unavailable {
		t.Fatal("expected unavailable flag")
	}
	if resp.Error != util.ErrUnsupportedLocation.Error() {
		t.Fatalf("expected unsupported location error")
	}
}

func TestTripGetRegionBounds_InternalError(t *testing.T) {
	svc := buildTripService(&tripRepoStub{
		getRegionBoundsFn: func(ctx context.Context, pickupCoords orb.Point) (*models.RegionModel, error) {
			return nil, fmt.Errorf("db error")
		},
	}, nil, nil)

	_, err := svc.GetRegionBounds(context.Background(), &pb.RegionBoundsRequest{
		Pickup: &pb.Coordinate{Longitude: 1.0, Latitude: 2.0},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", grpcCode(err))
	}
}

func TestTripStartTrip_Success(t *testing.T) {
	trip := makeCompletedTrip()
	trip.Status = types.TripStatusSearching
	trip.UserID = bson.NewObjectID()

	bus := &mockBus{}
	var publishCalls []messaging.AmqpEvent
	bus.publishFn = func(ctx context.Context, exchange messaging.AmqpExchange, routingKey messaging.AmqpEvent, msg messaging.AmqpMessage) error {
		publishCalls = append(publishCalls, routingKey)
		return nil
	}

	svc := buildTripService(&tripRepoStub{
		createTripFn: func(ctx context.Context, fareId, userId string) (*models.TripModel, error) {
			if fareId != "fare123" {
				t.Fatalf("unexpected fare id: %s", fareId)
			}
			if userId != "user456" {
				t.Fatalf("unexpected user id: %s", userId)
			}
			return trip, nil
		},
	}, nil, bus)

	resp, err := svc.StartTrip(context.Background(), &pb.StartTripRequest{
		RideFareId: "fare123",
		UserId:     "user456",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TripId != trip.ID.Hex() {
		t.Fatalf("expected trip id %s, got %s", trip.ID.Hex(), resp.TripId)
	}

	foundTripCreated := false
	for _, c := range publishCalls {
		if c == messaging.TripEventCreated {
			foundTripCreated = true
		}
	}
	if !foundTripCreated {
		t.Fatalf("expected TripEventCreated publish call, got calls: %v", publishCalls)
	}
}

func TestTripStartTrip_SessionExpired(t *testing.T) {
	svc := buildTripService(&tripRepoStub{
		createTripFn: func(ctx context.Context, fareId, userId string) (*models.TripModel, error) {
			return nil, util.ErrTripSessionExpired
		},
	}, nil, nil)

	_, err := svc.StartTrip(context.Background(), &pb.StartTripRequest{
		RideFareId: "expired",
		UserId:     "user1",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", grpcCode(err))
	}
}

func TestTripStartTrip_CreateError(t *testing.T) {
	svc := buildTripService(&tripRepoStub{
		createTripFn: func(ctx context.Context, fareId, userId string) (*models.TripModel, error) {
			return nil, fmt.Errorf("db error")
		},
	}, nil, nil)

	_, err := svc.StartTrip(context.Background(), &pb.StartTripRequest{
		RideFareId: "fare123",
		UserId:     "user1",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", grpcCode(err))
	}
}

func TestTripStartTrip_PublishError(t *testing.T) {
	trip := makeCompletedTrip()
	trip.Status = types.TripStatusSearching

	bus := &mockBus{
		publishFn: func(ctx context.Context, exchange messaging.AmqpExchange, routingKey messaging.AmqpEvent, msg messaging.AmqpMessage) error {
			return fmt.Errorf("queue error")
		},
	}

	svc := buildTripService(&tripRepoStub{
		createTripFn: func(ctx context.Context, fareId, userId string) (*models.TripModel, error) {
			return trip, nil
		},
	}, nil, bus)

	_, err := svc.StartTrip(context.Background(), &pb.StartTripRequest{
		RideFareId: "fare123",
		UserId:     "user1",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", grpcCode(err))
	}
}

func TestTripPreview_UnratedTrip(t *testing.T) {
	unrated := makeUnratedTrip()

	var publishedData json.RawMessage
	bus := &mockBus{
		publishFn: func(ctx context.Context, exchange messaging.AmqpExchange, routingKey messaging.AmqpEvent, msg messaging.AmqpMessage) error {
			publishedData = msg.Data
			return nil
		},
	}

	svc := buildTripService(&tripRepoStub{
		getLastUnratedTripFn: func(ctx context.Context, userId string) (*models.TripModel, error) {
			if userId != "rider-1" {
				t.Fatalf("unexpected user id: %s", userId)
			}
			return unrated, nil
		},
	}, nil, bus)

	_, err := svc.PreviewTrip(context.Background(), &pb.PreviewTripRequest{
		UserId:      "rider-1",
		RegionId:    "reg-1",
		Pickup:      &pb.Coordinate{Longitude: 3.0, Latitude: 6.0},
		Destination: &pb.Coordinate{Longitude: 4.0, Latitude: 7.0},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v", grpcCode(err))
	}
	if grpcMessage(err) != util.ErrTripRatingRequired.Error() {
		t.Fatalf("expected trip rating required message, got %s", grpcMessage(err))
	}

	if publishedData == nil {
		t.Fatal("expected rating-required message to be published")
	}

	var wsMsg contracts.WebsocketMessage
	json.Unmarshal(publishedData, &wsMsg)
	if wsMsg.Type != string(messaging.TripEventRatingRequired) {
		t.Fatalf("expected TripEventRatingRequired type, got %s", wsMsg.Type)
	}
}

func TestTripPreview_GetLastUnratedTripError(t *testing.T) {
	svc := buildTripService(&tripRepoStub{
		getLastUnratedTripFn: func(ctx context.Context, userId string) (*models.TripModel, error) {
			return nil, fmt.Errorf("db error")
		},
	}, nil, nil)

	_, err := svc.PreviewTrip(context.Background(), &pb.PreviewTripRequest{
		UserId:      "rider-1",
		RegionId:    "reg-1",
		Pickup:      &pb.Coordinate{Longitude: 3.0, Latitude: 6.0},
		Destination: &pb.Coordinate{Longitude: 4.0, Latitude: 7.0},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", grpcCode(err))
	}
}

func TestTripPreview_OsrmError(t *testing.T) {
	osrmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer osrmSrv.Close()

	svc := buildTripService(&tripRepoStub{
		getLastUnratedTripFn: func(ctx context.Context, userId string) (*models.TripModel, error) {
			return nil, nil
		},
	}, nil, nil)
	svc.httpClient = &http.Client{Transport: &interceptRoundTripper{osrmURL: osrmSrv.URL}}

	_, err := svc.PreviewTrip(context.Background(), &pb.PreviewTripRequest{
		UserId:      "rider-1",
		RegionId:    "reg-1",
		Pickup:      &pb.Coordinate{Longitude: 3.0, Latitude: 6.0},
		Destination: &pb.Coordinate{Longitude: 4.0, Latitude: 7.0},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", grpcCode(err))
	}
}

func TestTripPreview_FullSuccess(t *testing.T) {
	osrmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(osrmResponseJSON()))
	}))
	defer osrmSrv.Close()

	weatherSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(weatherResponseJSON(800)))
	}))
	defer weatherSrv.Close()

	cache := newTestCache("active_driver:d1", "active_driver:d2")
	pricing := makePricingModels()
	storeCalled := false

	svc := buildTripService(&tripRepoStub{
		getLastUnratedTripFn: func(ctx context.Context, userId string) (*models.TripModel, error) {
			return nil, nil
		},
		getPricingPerRegionFn: func(ctx context.Context, regionId string) ([]*models.PricingModel, error) {
			if regionId != "reg-1" {
				t.Fatalf("unexpected region: %s", regionId)
			}
			return pricing, nil
		},
		getActiveRequestsFn: func(ctx context.Context, pickupCoords orb.Point) (int64, error) {
			return 3, nil
		},
		storeRideFaresFn: func(ctx context.Context, rideFares []*pb.RideFare, route models.RouteDetails, userId, regionId string) error {
			storeCalled = true
			if len(rideFares) != 2 {
				t.Fatalf("expected 2 fare entries, got %d", len(rideFares))
			}
			return nil
		},
	}, cache, nil)
	svc.httpClient = &http.Client{Transport: &interceptRoundTripper{osrmURL: osrmSrv.URL, weatherURL: weatherSrv.URL}}

	resp, err := svc.PreviewTrip(context.Background(), &pb.PreviewTripRequest{
		UserId:      "rider-1",
		RegionId:    "reg-1",
		Pickup:      &pb.Coordinate{Longitude: 3.3792, Latitude: 6.5244},
		Destination: &pb.Coordinate{Longitude: 3.4064, Latitude: 6.4654},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.RideFares) != 2 {
		t.Fatalf("expected 2 ride fares, got %d", len(resp.RideFares))
	}
	if !storeCalled {
		t.Fatal("expected StoreRideFares to be called")
	}
}

func TestTripPreview_PricingError(t *testing.T) {
	osrmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(osrmResponseJSON()))
	}))
	defer osrmSrv.Close()

	svc := buildTripService(&tripRepoStub{
		getLastUnratedTripFn: func(ctx context.Context, userId string) (*models.TripModel, error) {
			return nil, nil
		},
		getPricingPerRegionFn: func(ctx context.Context, regionId string) ([]*models.PricingModel, error) {
			return nil, fmt.Errorf("db error")
		},
	}, nil, nil)
	svc.httpClient = &http.Client{Transport: &interceptRoundTripper{osrmURL: osrmSrv.URL}}

	_, err := svc.PreviewTrip(context.Background(), &pb.PreviewTripRequest{
		UserId:      "rider-1",
		RegionId:    "reg-1",
		Pickup:      &pb.Coordinate{Longitude: 3.3792, Latitude: 6.5244},
		Destination: &pb.Coordinate{Longitude: 3.4064, Latitude: 6.4654},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", grpcCode(err))
	}
}

func TestTripPreview_StoreRideFaresError(t *testing.T) {
	osrmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(osrmResponseJSON()))
	}))
	defer osrmSrv.Close()

	weatherSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(weatherResponseJSON(800)))
	}))
	defer weatherSrv.Close()

	cache := newTestCache("active_driver:d1")
	pricing := makePricingModels()

	svc := buildTripService(&tripRepoStub{
		getLastUnratedTripFn: func(ctx context.Context, userId string) (*models.TripModel, error) {
			return nil, nil
		},
		getPricingPerRegionFn: func(ctx context.Context, regionId string) ([]*models.PricingModel, error) {
			return pricing, nil
		},
		getActiveRequestsFn: func(ctx context.Context, pickupCoords orb.Point) (int64, error) {
			return 1, nil
		},
		storeRideFaresFn: func(ctx context.Context, rideFares []*pb.RideFare, route models.RouteDetails, userId, regionId string) error {
			return fmt.Errorf("db write error")
		},
	}, cache, nil)
	svc.httpClient = &http.Client{Transport: &interceptRoundTripper{osrmURL: osrmSrv.URL, weatherURL: weatherSrv.URL}}

	_, err := svc.PreviewTrip(context.Background(), &pb.PreviewTripRequest{
		UserId:      "rider-1",
		RegionId:    "reg-1",
		Pickup:      &pb.Coordinate{Longitude: 3.3792, Latitude: 6.5244},
		Destination: &pb.Coordinate{Longitude: 3.4064, Latitude: 6.4654},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", grpcCode(err))
	}
}

func TestTripServiceInterface(t *testing.T) {
	var _ pb.TripServiceServer = (*TripService)(nil)
}
