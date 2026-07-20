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
	repo "github.com/xerdin442/wayfare/services/payment/internal/infra/repository"
	"github.com/xerdin442/wayfare/services/payment/internal/secrets"
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

type paymentRepoStub struct {
	getTripByIDFn           func(ctx context.Context, tripId string) (*models.TripModel, error)
	getDriverByIDFn         func(ctx context.Context, driverId string) (*models.DriverModel, error)
	getTransactionByFilter  func(ctx context.Context, id string) (*models.TransactionModel, error)
	createTransactionFn     func(ctx context.Context, data *repo.CreateTransactionData) (string, error)
	updateTransactionFn     func(ctx context.Context, txnId string, status types.PaymentStatus, provider types.PaymentProvider) error
}

func (s *paymentRepoStub) GetTripByID(ctx context.Context, tripId string) (*models.TripModel, error) {
	return s.getTripByIDFn(ctx, tripId)
}
func (s *paymentRepoStub) GetDriverByID(ctx context.Context, driverId string) (*models.DriverModel, error) {
	return s.getDriverByIDFn(ctx, driverId)
}
func (s *paymentRepoStub) GetTransactionByFilterID(ctx context.Context, id string) (*models.TransactionModel, error) {
	return s.getTransactionByFilter(ctx, id)
}
func (s *paymentRepoStub) CreateTransaction(ctx context.Context, data *repo.CreateTransactionData) (string, error) {
	return s.createTransactionFn(ctx, data)
}
func (s *paymentRepoStub) UpdateTransaction(ctx context.Context, txnId string, status types.PaymentStatus, provider types.PaymentProvider) error {
	return s.updateTransactionFn(ctx, txnId, status, provider)
}

type paymentCacheStub struct {
	existsFn func(ctx context.Context, keys ...string) *redis.IntCmd
	setFn    func(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
}

func (c *paymentCacheStub) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	return c.existsFn(ctx, keys...)
}
func (c *paymentCacheStub) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	return c.setFn(ctx, key, value, expiration)
}

type payBus struct {
	publishFn func(ctx context.Context, exchange messaging.AmqpExchange, routingKey messaging.AmqpEvent, msg messaging.AmqpMessage) error
	calls     []messaging.AmqpEvent
}

func (m *payBus) PublishMessage(ctx context.Context, exchange messaging.AmqpExchange, routingKey messaging.AmqpEvent, msg messaging.AmqpMessage) error {
	m.calls = append(m.calls, routingKey)
	if m.publishFn != nil {
		return m.publishFn(ctx, exchange, routingKey, msg)
	}
	return nil
}
func (m *payBus) ConsumeMessages(queueName messaging.AmqpQueue, handler func(context.Context, amqp.Delivery) error) error {
	return nil
}

func grpcCode(err error) codes.Code {
	st, ok := status.FromError(err)
	if !ok {
		return codes.OK
	}
	return st.Code()
}

func buildPaymentService(repo *paymentRepoStub, cache *paymentCacheStub, bus *payBus) *PaymentService {
	if bus == nil {
		bus = &payBus{}
	}
	return &PaymentService{
		repo:       repo,
		cache:      cache,
		bus:        bus,
		env:        &secrets.Secrets{PaystackApiUrl: "http://paystack.test", PaystackSecretKey: "ps-secret", FlutterwaveApiUrl: "http://flutterwave.test", FlutterwaveSecretKey: "fw-secret"},
		httpClient: http.DefaultClient,
	}
}

func tripModel(rideFare int64) *models.TripModel {
	return &models.TripModel{
		ID:       bson.NewObjectID(),
		UserID:   bson.NewObjectID(),
		RideFare: rideFare,
		Region:   "Lagos",
		Route: models.RouteDetails{
			Pickup:      models.GeoPoint{Coordinates: orb.Point{3.0, 6.0}},
			Destination: models.GeoPoint{Coordinates: orb.Point{4.0, 7.0}},
			Addresses:   []string{"A", "B"},
			Duration:    1800,
			Distance:    12000,
		},
	}
}

func driverModel(returns int64) *models.DriverModel {
	return &models.DriverModel{
		ID:                 bson.NewObjectID(),
		OutstandingReturns: returns,
	}
}

func txnModel(id bson.ObjectID, amount int64, status types.PaymentStatus) *models.TransactionModel {
	return &models.TransactionModel{
		ID:     id,
		Amount: amount,
		Status: status,
	}
}

func cacheExistsCmd(val int64, withErr error) *paymentCacheStub {
	return &paymentCacheStub{
		existsFn: func(ctx context.Context, keys ...string) *redis.IntCmd {
			cmd := redis.NewIntCmd(ctx)
			if withErr != nil {
				cmd.SetErr(withErr)
			} else {
				cmd.SetVal(val)
			}
			return cmd
		},
		setFn: func(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
			return redis.NewStatusCmd(ctx)
		},
	}
}

type interceptRoundTripper struct {
	paystackURL    string
	flutterwaveURL string
}

func (rt *interceptRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	if strings.Contains(clone.URL.Host, "paystack") && rt.paystackURL != "" {
		clone.URL.Scheme = "http"
		clone.URL.Host = strings.TrimPrefix(rt.paystackURL, "http://")
	} else if strings.Contains(clone.URL.Host, "flutterwave") && rt.flutterwaveURL != "" {
		clone.URL.Scheme = "http"
		clone.URL.Host = strings.TrimPrefix(rt.flutterwaveURL, "http://")
	}
	return http.DefaultTransport.RoundTrip(clone)
}

func paystackOK() string {
	return `{"status":true,"message":"success","data":{"authorization_url":"https://checkout.paystack.com/test","reference":"ref-123","access_code":"abc"}}`
}
func paystackError() string {
	return `{"status":false,"message":"unavailable","type":"api_error","code":"500"}`
}
func flutterwaveOK() string {
	return `{"status":true,"message":"success","data":{"link":"https://checkout.flutterwave.com/test"}}`
}
func flutterwaveError() string {
	return `{"status":false,"message":"unavailable","error_id":"err-1","code":"500"}`
}

func newPaystackOK(t *testing.T) (svr *httptest.Server, transport *interceptRoundTripper) {
	svr = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(paystackOK()))
	}))
	transport = &interceptRoundTripper{paystackURL: svr.URL}
	return
}

func newPaystack500(t *testing.T) (svr *httptest.Server, transport *interceptRoundTripper) {
	svr = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(paystackError()))
	}))
	transport = &interceptRoundTripper{paystackURL: svr.URL}
	return
}

func newFlutterwaveOK(t *testing.T) (svr *httptest.Server, transport *interceptRoundTripper) {
	svr = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(flutterwaveOK()))
	}))
	transport = &interceptRoundTripper{flutterwaveURL: svr.URL}
	return
}

func newFlutterwave500(t *testing.T) (svr *httptest.Server, transport *interceptRoundTripper) {
	svr = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(flutterwaveError()))
	}))
	transport = &interceptRoundTripper{flutterwaveURL: svr.URL}
	return
}

// =============================================================================
// InitiateCheckout - ride fare path
// =============================================================================

func TestPayInitiate_RideFare_TripNotFound(t *testing.T) {
	psSrv, tr := newPaystackOK(t)
	defer psSrv.Close()

	trip := tripModel(500000)
	svc := buildPaymentService(&paymentRepoStub{
		getTripByIDFn: func(ctx context.Context, tripId string) (*models.TripModel, error) {
			if tripId != trip.ID.Hex() {
				t.Fatalf("unexpected trip id: %s", tripId)
			}
			return nil, util.ErrDocumentNotFound
		},
	}, cacheExistsCmd(0, nil), nil)
	svc.httpClient = &http.Client{Transport: tr}

	_, err := svc.InitiateCheckout(context.Background(), &pb.InitiateCheckoutRequest{
		TripId:  trip.ID.Hex(),
		TxnType: string(types.TransactionRideFare),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", grpcCode(err))
	}
}

func TestPayInitiate_RideFare_TripRepoError(t *testing.T) {
	psSrv, tr := newPaystackOK(t)
	defer psSrv.Close()

	trip := tripModel(500000)
	svc := buildPaymentService(&paymentRepoStub{
		getTripByIDFn: func(ctx context.Context, tripId string) (*models.TripModel, error) {
			return nil, fmt.Errorf("db error")
		},
	}, cacheExistsCmd(0, nil), nil)
	svc.httpClient = &http.Client{Transport: tr}

	_, err := svc.InitiateCheckout(context.Background(), &pb.InitiateCheckoutRequest{
		TripId:  trip.ID.Hex(),
		TxnType: string(types.TransactionRideFare),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", grpcCode(err))
	}
}

func TestPayInitiate_RideFare_ExistingPendingTxn(t *testing.T) {
	psSrv, tr := newPaystackOK(t)
	defer psSrv.Close()

	trip := tripModel(500000)
	existing := txnModel(bson.NewObjectID(), 500000, types.PaymentStatusPending)

	svc := buildPaymentService(&paymentRepoStub{
		getTripByIDFn: func(ctx context.Context, tripId string) (*models.TripModel, error) {
			return trip, nil
		},
		getTransactionByFilter: func(ctx context.Context, id string) (*models.TransactionModel, error) {
			return existing, nil
		},
	}, cacheExistsCmd(0, nil), nil)
	svc.httpClient = &http.Client{Transport: tr}

	resp, err := svc.InitiateCheckout(context.Background(), &pb.InitiateCheckoutRequest{
		TripId:  trip.ID.Hex(),
		UserId:  "user-1",
		Email:   "user@test.com",
		TxnType: string(types.TransactionRideFare),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CheckoutUrl != "https://checkout.paystack.com/test" {
		t.Fatalf("expected paystack checkout url, got %s", resp.CheckoutUrl)
	}
}

func TestPayInitiate_RideFare_NewTxn(t *testing.T) {
	psSrv, tr := newPaystackOK(t)
	defer psSrv.Close()

	trip := tripModel(500000)
	txnID := bson.NewObjectID().Hex()

	svc := buildPaymentService(&paymentRepoStub{
		getTripByIDFn: func(ctx context.Context, tripId string) (*models.TripModel, error) {
			return trip, nil
		},
		getTransactionByFilter: func(ctx context.Context, id string) (*models.TransactionModel, error) {
			return nil, nil
		},
		createTransactionFn: func(ctx context.Context, data *repo.CreateTransactionData) (string, error) {
			if data.Amount != trip.RideFare {
				t.Fatalf("expected amount %d, got %d", trip.RideFare, data.Amount)
			}
			if data.Type != types.TransactionRideFare {
				t.Fatalf("expected ride_fare type")
			}
			return txnID, nil
		},
	}, cacheExistsCmd(0, nil), nil)
	svc.httpClient = &http.Client{Transport: tr}

	resp, err := svc.InitiateCheckout(context.Background(), &pb.InitiateCheckoutRequest{
		TripId:  trip.ID.Hex(),
		UserId:  "user-1",
		Email:   "user@test.com",
		TxnType: string(types.TransactionRideFare),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CheckoutUrl != "https://checkout.paystack.com/test" {
		t.Fatalf("expected checkout url, got %s", resp.CheckoutUrl)
	}
}

func TestPayInitiate_RideFare_TxnFetchError(t *testing.T) {
	psSrv, tr := newPaystackOK(t)
	defer psSrv.Close()

	trip := tripModel(500000)
	svc := buildPaymentService(&paymentRepoStub{
		getTripByIDFn: func(ctx context.Context, tripId string) (*models.TripModel, error) {
			return trip, nil
		},
		getTransactionByFilter: func(ctx context.Context, id string) (*models.TransactionModel, error) {
			return nil, fmt.Errorf("query error")
		},
	}, cacheExistsCmd(0, nil), nil)
	svc.httpClient = &http.Client{Transport: tr}

	_, err := svc.InitiateCheckout(context.Background(), &pb.InitiateCheckoutRequest{
		TripId:  trip.ID.Hex(),
		TxnType: string(types.TransactionRideFare),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", grpcCode(err))
	}
}

func TestPayInitiate_RideFare_TxnCreateError(t *testing.T) {
	psSrv, tr := newPaystackOK(t)
	defer psSrv.Close()

	trip := tripModel(500000)
	svc := buildPaymentService(&paymentRepoStub{
		getTripByIDFn: func(ctx context.Context, tripId string) (*models.TripModel, error) {
			return trip, nil
		},
		getTransactionByFilter: func(ctx context.Context, id string) (*models.TransactionModel, error) {
			return nil, nil
		},
		createTransactionFn: func(ctx context.Context, data *repo.CreateTransactionData) (string, error) {
			return "", fmt.Errorf("insert error")
		},
	}, cacheExistsCmd(0, nil), nil)
	svc.httpClient = &http.Client{Transport: tr}

	_, err := svc.InitiateCheckout(context.Background(), &pb.InitiateCheckoutRequest{
		TripId:  trip.ID.Hex(),
		TxnType: string(types.TransactionRideFare),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", grpcCode(err))
	}
}

// =============================================================================
// InitiateCheckout - returns path
// =============================================================================

func TestPayInitiate_Returns_DriverRepoError(t *testing.T) {
	psSrv, tr := newPaystackOK(t)
	defer psSrv.Close()

	svc := buildPaymentService(&paymentRepoStub{
		getDriverByIDFn: func(ctx context.Context, driverId string) (*models.DriverModel, error) {
			return nil, fmt.Errorf("db error")
		},
	}, cacheExistsCmd(0, nil), nil)
	svc.httpClient = &http.Client{Transport: tr}

	_, err := svc.InitiateCheckout(context.Background(), &pb.InitiateCheckoutRequest{
		UserId:  "driver-1",
		TxnType: string(types.TransactionReturns),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", grpcCode(err))
	}
}

func TestPayInitiate_Returns_NewTxn(t *testing.T) {
	psSrv, tr := newPaystackOK(t)
	defer psSrv.Close()

	driver := driverModel(300000)

	svc := buildPaymentService(&paymentRepoStub{
		getDriverByIDFn: func(ctx context.Context, driverId string) (*models.DriverModel, error) {
			return driver, nil
		},
		getTransactionByFilter: func(ctx context.Context, id string) (*models.TransactionModel, error) {
			return nil, nil
		},
		createTransactionFn: func(ctx context.Context, data *repo.CreateTransactionData) (string, error) {
			if data.Amount != driver.OutstandingReturns {
				t.Fatalf("expected amount %d, got %d", driver.OutstandingReturns, data.Amount)
			}
			if data.Type != types.TransactionReturns {
				t.Fatalf("expected returns type")
			}
			return bson.NewObjectID().Hex(), nil
		},
	}, cacheExistsCmd(0, nil), nil)
	svc.httpClient = &http.Client{Transport: tr}

	resp, err := svc.InitiateCheckout(context.Background(), &pb.InitiateCheckoutRequest{
		UserId:  "driver-1",
		Email:   "driver@test.com",
		TxnType: string(types.TransactionReturns),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CheckoutUrl != "https://checkout.paystack.com/test" {
		t.Fatalf("expected checkout url, got %s", resp.CheckoutUrl)
	}
}

// =============================================================================
// resolvePaymentProvider tests
// =============================================================================

func TestPayResolveProvider_DefaultPaystack(t *testing.T) {
	psSrv, tr := newPaystackOK(t)
	defer psSrv.Close()

	trip := tripModel(500000)
	txn := txnModel(bson.NewObjectID(), 500000, types.PaymentStatusPending)

	svc := buildPaymentService(&paymentRepoStub{
		getTripByIDFn: func(ctx context.Context, tripId string) (*models.TripModel, error) {
			return trip, nil
		},
		getTransactionByFilter: func(ctx context.Context, id string) (*models.TransactionModel, error) {
			return txn, nil
		},
	}, cacheExistsCmd(0, nil), nil)
	svc.httpClient = &http.Client{Transport: tr}

	resp, err := svc.InitiateCheckout(context.Background(), &pb.InitiateCheckoutRequest{
		TripId:  trip.ID.Hex(),
		UserId:  "user-1",
		Email:   "user@test.com",
		TxnType: string(types.TransactionRideFare),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CheckoutUrl != "https://checkout.paystack.com/test" {
		t.Fatalf("expected paystack url, got %s", resp.CheckoutUrl)
	}
}

func TestPayResolveProvider_CacheError_DefaultsPaystack(t *testing.T) {
	psSrv, tr := newPaystackOK(t)
	defer psSrv.Close()

	trip := tripModel(500000)
	txn := txnModel(bson.NewObjectID(), 500000, types.PaymentStatusPending)

	cache := &paymentCacheStub{
		existsFn: func(ctx context.Context, keys ...string) *redis.IntCmd {
			cmd := redis.NewIntCmd(ctx)
			cmd.SetErr(fmt.Errorf("redis down"))
			return cmd
		},
	}

	svc := buildPaymentService(&paymentRepoStub{
		getTripByIDFn: func(ctx context.Context, tripId string) (*models.TripModel, error) {
			return trip, nil
		},
		getTransactionByFilter: func(ctx context.Context, id string) (*models.TransactionModel, error) {
			return txn, nil
		},
	}, cache, nil)
	svc.httpClient = &http.Client{Transport: tr}

	resp, err := svc.InitiateCheckout(context.Background(), &pb.InitiateCheckoutRequest{
		TripId:  trip.ID.Hex(),
		UserId:  "user-1",
		Email:   "user@test.com",
		TxnType: string(types.TransactionRideFare),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CheckoutUrl != "https://checkout.paystack.com/test" {
		t.Fatalf("expected paystack url, got %s", resp.CheckoutUrl)
	}
}

func TestPayResolveProvider_Flutterwave(t *testing.T) {
	fwSrv, fwtr := newFlutterwaveOK(t)
	defer fwSrv.Close()

	trip := tripModel(500000)
	txn := txnModel(bson.NewObjectID(), 500000, types.PaymentStatusPending)

	svc := buildPaymentService(&paymentRepoStub{
		getTripByIDFn: func(ctx context.Context, tripId string) (*models.TripModel, error) {
			return trip, nil
		},
		getTransactionByFilter: func(ctx context.Context, id string) (*models.TransactionModel, error) {
			return txn, nil
		},
	}, cacheExistsCmd(1, nil), nil)
	svc.httpClient = &http.Client{Transport: fwtr}

	resp, err := svc.InitiateCheckout(context.Background(), &pb.InitiateCheckoutRequest{
		TripId:  trip.ID.Hex(),
		UserId:  "user-1",
		Email:   "user@test.com",
		TxnType: string(types.TransactionRideFare),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CheckoutUrl != "https://checkout.flutterwave.com/test" {
		t.Fatalf("expected flutterwave url, got %s", resp.CheckoutUrl)
	}
}

// =============================================================================
// Gateway failover tests
// =============================================================================

func TestPayFallback_PaystackUnavailable_FallbackToFlutterwave(t *testing.T) {
	psSrv, psTr := newPaystack500(t)
	defer psSrv.Close()
	fwSrv, fwTr := newFlutterwaveOK(t)
	defer fwSrv.Close()

	trip := tripModel(500000)
	txn := txnModel(bson.NewObjectID(), 500000, types.PaymentStatusPending)

	cache := &paymentCacheStub{
		existsFn: func(ctx context.Context, keys ...string) *redis.IntCmd {
			return redis.NewIntCmd(ctx)
		},
		setFn: func(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
			return redis.NewStatusCmd(ctx)
		},
	}

	svc := buildPaymentService(&paymentRepoStub{
		getTripByIDFn: func(ctx context.Context, tripId string) (*models.TripModel, error) {
			return trip, nil
		},
		getTransactionByFilter: func(ctx context.Context, id string) (*models.TransactionModel, error) {
			return txn, nil
		},
	}, cache, nil)
	svc.httpClient = &http.Client{Transport: &interceptRoundTripper{
		paystackURL:    psTr.paystackURL,
		flutterwaveURL: fwTr.flutterwaveURL,
	}}

	resp, err := svc.InitiateCheckout(context.Background(), &pb.InitiateCheckoutRequest{
		TripId:  trip.ID.Hex(),
		UserId:  "user-1",
		Email:   "user@test.com",
		TxnType: string(types.TransactionRideFare),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CheckoutUrl != "https://checkout.flutterwave.com/test" {
		t.Fatalf("expected flutterwave fallback url, got %s", resp.CheckoutUrl)
	}
}

func TestPayFallback_AllGatewaysDown(t *testing.T) {
	psSrv, psTr := newPaystack500(t)
	defer psSrv.Close()
	fwSrv, fwTr := newFlutterwave500(t)
	defer fwSrv.Close()

	trip := tripModel(500000)
	txn := txnModel(bson.NewObjectID(), 500000, types.PaymentStatusPending)

	abortCalled := false
	bus := &payBus{}

	cache := &paymentCacheStub{
		existsFn: func(ctx context.Context, keys ...string) *redis.IntCmd {
			return redis.NewIntCmd(ctx)
		},
		setFn: func(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
			return redis.NewStatusCmd(ctx)
		},
	}

	svc := buildPaymentService(&paymentRepoStub{
		getTripByIDFn: func(ctx context.Context, tripId string) (*models.TripModel, error) {
			return trip, nil
		},
		getTransactionByFilter: func(ctx context.Context, id string) (*models.TransactionModel, error) {
			return txn, nil
		},
		updateTransactionFn: func(ctx context.Context, txnId string, st types.PaymentStatus, prov types.PaymentProvider) error {
			abortCalled = true
			return nil
		},
	}, cache, bus)
	svc.httpClient = &http.Client{Transport: &interceptRoundTripper{
		paystackURL:    psTr.paystackURL,
		flutterwaveURL: fwTr.flutterwaveURL,
	}}

	_, err := svc.InitiateCheckout(context.Background(), &pb.InitiateCheckoutRequest{
		TripId:  trip.ID.Hex(),
		UserId:  "user-1",
		Email:   "user@test.com",
		TxnType: string(types.TransactionRideFare),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.Unavailable {
		t.Fatalf("expected Unavailable, got %v", grpcCode(err))
	}
	if !abortCalled {
		t.Fatal("expected UpdateTransaction to be called for abort")
	}
}

// =============================================================================
// buildCheckoutPayloads tests
// =============================================================================

func TestPayBuildCheckoutPayloads(t *testing.T) {
	svc := buildPaymentService(nil, nil, nil)

	ps, fw, err := svc.buildCheckoutPayloads(&pb.InitiateCheckoutRequest{
		Email:        "user@test.com",
		TripRating:   4,
		RiderComment: "great",
		DriverTip:    200,
	}, "txn-123", 500000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ps.Email != "user@test.com" {
		t.Fatalf("expected email")
	}
	if ps.Amount != 520000 {
		t.Fatalf("expected kobo amount 520000 (500000 + 20000 tip), got %d", ps.Amount)
	}
	if ps.Reference != "txn-123" {
		t.Fatalf("expected txn reference")
	}
	if fw.Amount != 5200 {
		t.Fatalf("expected naira amount 5200 (5000 + 200), got %d", fw.Amount)
	}
	if fw.Customer.Email != "user@test.com" {
		t.Fatalf("expected customer email")
	}

	var meta contracts.PaymentMetadata
	json.Unmarshal([]byte(ps.Metadata), &meta)
	if meta.TripRating != 4 {
		t.Fatalf("expected rating 4")
	}
	if meta.RiderComment != "great" {
		t.Fatalf("expected comment")
	}
	if meta.DriverTip != 20000 {
		t.Fatalf("expected driver tip 20000 kobo, got %d", meta.DriverTip)
	}
}

// =============================================================================
// Interface compliance
// =============================================================================

func TestPaymentServiceInterface(t *testing.T) {
	var _ pb.PaymentServiceServer = (*PaymentService)(nil)
}
