package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/client"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestHandleTripPreview_EmptyBody(t *testing.T) {
	c, w := setupAuthTestContext("rider-123", types.RoleRider)
	c.Request.Method = "POST"
	c.Request.Header.Set("Content-Type", "application/json")
	h := buildTestHandler(t, nil)
	h.HandleTripPreview(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleTripPreview_InvalidJSON(t *testing.T) {
	c, w := setupAuthTestContext("rider-123", types.RoleRider)
	c.Request.Method = "POST"
	c.Request.Header.Set("Content-Type", "application/json")
	h := buildTestHandler(t, nil)
	h.HandleTripPreview(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleStartTrip_EmptyBody(t *testing.T) {
	c, w := setupAuthTestContext("rider-123", types.RoleRider)
	c.Request.Method = "POST"
	c.Request.Header.Set("Content-Type", "application/json")
	h := buildTestHandler(t, nil)
	h.HandleStartTrip(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleStartTrip_InvalidJSON(t *testing.T) {
	c, w := setupAuthTestContext("rider-123", types.RoleRider)
	c.Request.Method = "POST"
	c.Request.Header.Set("Content-Type", "application/json")
	h := buildTestHandler(t, nil)
	h.HandleStartTrip(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// mockTripClient stubs GetTripDetails only. Calling any other method panics via the
// nil embedded interface.
type mockTripClient struct {
	pb.TripServiceClient
	details *pb.TripDetailsResponse
	err     error
}

func (m *mockTripClient) GetTripDetails(ctx context.Context, in *pb.TripDetailsRequest, opts ...grpc.CallOption) (*pb.TripDetailsResponse, error) {
	return m.details, m.err
}

func TestHandleTripChat_HiddenFromNonParticipants(t *testing.T) {
	cases := map[string]*mockTripClient{
		"trip not found":  {err: status.Error(codes.NotFound, "no document found")},
		"not on the trip": {details: &pb.TripDetailsResponse{UserId: "rider-1", DriverId: "driver-1"}},
	}

	for name, trip := range cases {
		t.Run(name, func(t *testing.T) {
			c, w := setupAuthTestContext("someone-else", types.RoleRider)
			c.Params = gin.Params{{Key: "id", Value: "trip-1"}}

			// The cache is nil, so reaching the chat history lookup would panic
			h := buildTestHandler(t, nil)
			h.cfg.Clients = &client.Registry{Trip: trip}
			h.HandleTripChat(c)

			if w.Code != http.StatusNotFound {
				t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}
