package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/client"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"google.golang.org/grpc"
)

var pngBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

// mockDriverClient stubs CheckEmailAvailability only. Calling any other method
// panics via the nil embedded interface.
type mockDriverClient struct {
	pb.DriverServiceClient
	emailAvailable bool
}

func (m *mockDriverClient) CheckEmailAvailability(ctx context.Context, in *pb.EmailAvailabilityRequest, opts ...grpc.CallOption) (*pb.EmailAvailabilityResponse, error) {
	return &pb.EmailAvailabilityResponse{Available: m.emailAvailable}, nil
}

// mockRiderSignupClient records the signup request it receives
type mockRiderSignupClient struct {
	pb.RiderServiceClient
	received *pb.SignupRiderRequest
}

func (m *mockRiderSignupClient) Signup(ctx context.Context, in *pb.SignupRiderRequest, opts ...grpc.CallOption) (*pb.AuthResponse, error) {
	m.received = in
	return &pb.AuthResponse{UserId: "rider-1"}, nil
}

type signupFile struct {
	field   string
	content []byte
}

func newSignupContext(t *testing.T, role string, fields map[string]string, files []signupFile) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range fields {
		w.WriteField(k, v)
	}
	for _, f := range files {
		part, err := w.CreateFormFile(f.field, "upload")
		if err != nil {
			t.Fatal(err)
		}
		part.Write(f.content)
	}
	w.Close()

	return newTestContext("POST", "/auth/signup", body.String(), map[string]string{
		"X-User-Role":  role,
		"Content-Type": w.FormDataContentType(),
	})
}

func driverFields() map[string]string {
	return map[string]string{
		"email":         "driver@test.com",
		"password":      "password123",
		"name":          "John Driver",
		"phone":         "08012345678",
		"carModel":      "Toyota Camry",
		"carColor":      "Black",
		"carPlate":      "ABC-123",
		"accountNumber": "0123456789",
		"accountName":   "John Driver",
		"bankName":      "Access Bank",
	}
}

func TestHandleSignup_DriverBadFileRejectedBeforeExternalCalls(t *testing.T) {
	c, w := newSignupContext(t, "driver", driverFields(), []signupFile{
		{"profileImage", pngBytes},
		{"verificationPhotos", []byte("%PDF-1.7 not an image")},
	})

	// No gRPC clients, cache or HTTP client are configured, so any external call would panic
	h := buildTestHandler(t, nil)
	h.HandleSignup(c)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSignup_DriverEmailTakenRejectedBeforePaystack(t *testing.T) {
	c, w := newSignupContext(t, "driver", driverFields(), []signupFile{
		{"profileImage", pngBytes},
		{"verificationPhotos", pngBytes},
	})

	// Cache and HTTP client are nil, so reaching the Paystack lookup would panic
	h := buildTestHandler(t, nil)
	h.cfg.Clients = &client.Registry{Driver: &mockDriverClient{emailAvailable: false}}
	h.HandleSignup(c)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSignup_DriverTooManyVerificationPhotos(t *testing.T) {
	files := []signupFile{{"profileImage", pngBytes}}
	for range 6 {
		files = append(files, signupFile{"verificationPhotos", pngBytes})
	}
	c, w := newSignupContext(t, "driver", driverFields(), files)

	h := buildTestHandler(t, nil)
	h.HandleSignup(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Errors map[string]string `json:"errors"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Errors["verificationPhotos"], "at most 5 items") {
		t.Fatalf("expected a verificationPhotos limit error, got %v", resp.Errors)
	}
}

func TestHandleSignup_RiderPhoneIsForwarded(t *testing.T) {
	fields := driverFields()
	c, w := newSignupContext(t, "rider", map[string]string{
		"email":    fields["email"],
		"password": fields["password"],
		"name":     fields["name"],
		"phone":    fields["phone"],
	}, nil)

	// A cancelled context makes the Cloudinary avatar copy fail immediately instead of
	// calling the network, which exercises the fallback to the DiceBear link
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Request = c.Request.WithContext(ctx)

	rider := &mockRiderSignupClient{}
	h := buildTestHandler(t, nil)
	h.cfg.Clients = &client.Registry{Rider: rider}
	h.HandleSignup(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if rider.received.Phone != fields["phone"] {
		t.Fatalf("expected phone %q to be forwarded, got %q", fields["phone"], rider.received.Phone)
	}
	if !strings.HasPrefix(rider.received.ProfileImage, "https://api.dicebear.com/9.x/notionists/png?seed=") {
		t.Fatalf("expected the DiceBear fallback avatar, got %q", rider.received.ProfileImage)
	}
}

func TestHandleSignup_BodyTooLarge(t *testing.T) {
	c, w := newSignupContext(t, "rider", map[string]string{"email": "rider@test.com"}, []signupFile{
		{"profileImage", make([]byte, maxSignupBodySize+1)},
	})

	h := buildTestHandler(t, nil)
	h.HandleSignup(c)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", w.Code, w.Body.String())
	}
}
