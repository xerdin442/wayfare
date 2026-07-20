package handlers

import (
	"net/http"
	"testing"

	"github.com/xerdin442/wayfare/shared/types"
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
