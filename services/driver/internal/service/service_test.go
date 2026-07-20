package service

import (
	"context"
	"testing"

	"github.com/xerdin442/wayfare/shared/models"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/types"
	"github.com/xerdin442/wayfare/shared/util"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type driverRepoStub struct {
	getByIDFn    func(ctx context.Context, id string) (*models.DriverModel, error)
	getByEmailFn func(ctx context.Context, email string) (*models.DriverModel, error)
	createFn     func(ctx context.Context, req *pb.SignupDriverRequest) (*models.DriverModel, error)
}

func (s *driverRepoStub) GetDriverByID(ctx context.Context, id string) (*models.DriverModel, error) {
	return s.getByIDFn(ctx, id)
}

func (s *driverRepoStub) GetDriverByEmail(ctx context.Context, email string) (*models.DriverModel, error) {
	return s.getByEmailFn(ctx, email)
}

func (s *driverRepoStub) CreateDriverAccount(ctx context.Context, req *pb.SignupDriverRequest) (*models.DriverModel, error) {
	return s.createFn(ctx, req)
}

func validDriverModel() *models.DriverModel {
	return &models.DriverModel{
		ID:                  bson.NewObjectID(),
		Name:                "John Driver",
		Email:               "driver@test.com",
		Phone:               "08012345678",
		Password:            mustHashPassword("password123"),
		ProfilePicture:      "https://example.com/photo.jpg",
		VerificationPhotos:  []string{"https://example.com/v1.jpg"},
		IsVerified:          true,
		CarPackage:          types.PackageSUV,
		CarPlate:            "ABC-123",
		CarModel:            "Toyota Camry",
		CarColor:            "Black",
		CurrentRating:       4.5,
		TotalCompletedTrips: 120,
		Tier:                types.TierGold,
	}
}

func mustHashPassword(pw string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return string(h)
}

func grpcCode(err error) codes.Code {
	st, ok := status.FromError(err)
	if !ok {
		return codes.OK
	}
	return st.Code()
}

func TestDriverGetProfile_Success(t *testing.T) {
	svc := &DriverService{
		repo: &driverRepoStub{
			getByIDFn: func(ctx context.Context, id string) (*models.DriverModel, error) {
				if id != "507f1f77bcf86cd799439011" {
					t.Fatalf("unexpected id: %s", id)
				}
				return validDriverModel(), nil
			},
		},
	}

	resp, err := svc.GetDriverProfile(context.Background(), &pb.GetProfileRequest{
		UserId: "507f1f77bcf86cd799439011",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d := resp.Driver
	if d.Name != "John Driver" {
		t.Fatalf("expected John Driver, got %s", d.Name)
	}
	if d.PackageSlug != "suv" {
		t.Fatalf("expected suv, got %s", d.PackageSlug)
	}
	if d.Tier != "gold" {
		t.Fatalf("expected gold tier, got %s", d.Tier)
	}
	if d.CurrentRating != 4.5 {
		t.Fatalf("expected 4.5, got %f", d.CurrentRating)
	}
}

func TestDriverGetProfile_NotFound(t *testing.T) {
	svc := &DriverService{
		repo: &driverRepoStub{
			getByIDFn: func(ctx context.Context, id string) (*models.DriverModel, error) {
				return nil, util.ErrDocumentNotFound
			},
		},
	}

	_, err := svc.GetDriverProfile(context.Background(), &pb.GetProfileRequest{
		UserId: "507f1f77bcf86cd799439011",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", grpcCode(err))
	}
}

func TestDriverGetProfile_InternalError(t *testing.T) {
	svc := &DriverService{
		repo: &driverRepoStub{
			getByIDFn: func(ctx context.Context, id string) (*models.DriverModel, error) {
				return nil, status.Error(codes.Internal, "db error")
			},
		},
	}

	_, err := svc.GetDriverProfile(context.Background(), &pb.GetProfileRequest{
		UserId: "507f1f77bcf86cd799439011",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", grpcCode(err))
	}
}

func TestDriverGetProfile_NotVerified(t *testing.T) {
	d := validDriverModel()
	d.IsVerified = false
	d.CarPackage = types.PackageDefault

	svc := &DriverService{
		repo: &driverRepoStub{
			getByIDFn: func(ctx context.Context, id string) (*models.DriverModel, error) {
				return d, nil
			},
		},
	}

	_, err := svc.GetDriverProfile(context.Background(), &pb.GetProfileRequest{
		UserId: "507f1f77bcf86cd799439011",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", grpcCode(err))
	}
}

func TestDriverGetProfile_OutstandingReturns(t *testing.T) {
	d := validDriverModel()
	d.OutstandingReturns = 5000

	svc := &DriverService{
		repo: &driverRepoStub{
			getByIDFn: func(ctx context.Context, id string) (*models.DriverModel, error) {
				return d, nil
			},
		},
	}

	_, err := svc.GetDriverProfile(context.Background(), &pb.GetProfileRequest{
		UserId: "507f1f77bcf86cd799439011",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", grpcCode(err))
	}
}

func TestDriverGetProfile_DefaultPackage(t *testing.T) {
	d := validDriverModel()
	d.CarPackage = types.PackageDefault

	svc := &DriverService{
		repo: &driverRepoStub{
			getByIDFn: func(ctx context.Context, id string) (*models.DriverModel, error) {
				return d, nil
			},
		},
	}

	_, err := svc.GetDriverProfile(context.Background(), &pb.GetProfileRequest{
		UserId: "507f1f77bcf86cd799439011",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", grpcCode(err))
	}
}

func TestDriverLogin_Success(t *testing.T) {
	d := validDriverModel()

	svc := &DriverService{
		repo: &driverRepoStub{
			getByEmailFn: func(ctx context.Context, email string) (*models.DriverModel, error) {
				if email != "driver@test.com" {
					t.Fatalf("unexpected email: %s", email)
				}
				return d, nil
			},
		},
	}

	resp, err := svc.Login(context.Background(), &pb.LoginRequest{
		Email:    "driver@test.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.UserId != d.ID.Hex() {
		t.Fatalf("expected user id %s, got %s", d.ID.Hex(), resp.UserId)
	}
}

func TestDriverLogin_NotFound(t *testing.T) {
	svc := &DriverService{
		repo: &driverRepoStub{
			getByEmailFn: func(ctx context.Context, email string) (*models.DriverModel, error) {
				return nil, nil
			},
		},
	}

	_, err := svc.Login(context.Background(), &pb.LoginRequest{
		Email:    "unknown@test.com",
		Password: "password123",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", grpcCode(err))
	}
}

func TestDriverLogin_WrongPassword(t *testing.T) {
	d := validDriverModel()

	svc := &DriverService{
		repo: &driverRepoStub{
			getByEmailFn: func(ctx context.Context, email string) (*models.DriverModel, error) {
				return d, nil
			},
		},
	}

	_, err := svc.Login(context.Background(), &pb.LoginRequest{
		Email:    "driver@test.com",
		Password: "wrongpassword",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", grpcCode(err))
	}
}

func TestDriverLogin_RepoError(t *testing.T) {
	svc := &DriverService{
		repo: &driverRepoStub{
			getByEmailFn: func(ctx context.Context, email string) (*models.DriverModel, error) {
				return nil, status.Error(codes.Internal, "db connection lost")
			},
		},
	}

	_, err := svc.Login(context.Background(), &pb.LoginRequest{
		Email:    "driver@test.com",
		Password: "password123",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", grpcCode(err))
	}
}

func TestDriverSignup_Success(t *testing.T) {
	created := validDriverModel()

	svc := &DriverService{
		repo: &driverRepoStub{
			getByEmailFn: func(ctx context.Context, email string) (*models.DriverModel, error) {
				return nil, nil
			},
			createFn: func(ctx context.Context, req *pb.SignupDriverRequest) (*models.DriverModel, error) {
				if req.Email != "newdriver@test.com" {
					t.Fatalf("unexpected email: %s", req.Email)
				}
				return created, nil
			},
		},
	}

	resp, err := svc.Signup(context.Background(), &pb.SignupDriverRequest{
		Email: "newdriver@test.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.UserId != created.ID.Hex() {
		t.Fatalf("expected user id %s, got %s", created.ID.Hex(), resp.UserId)
	}
}

func TestDriverSignup_AlreadyExists(t *testing.T) {
	svc := &DriverService{
		repo: &driverRepoStub{
			getByEmailFn: func(ctx context.Context, email string) (*models.DriverModel, error) {
				return validDriverModel(), nil
			},
		},
	}

	_, err := svc.Signup(context.Background(), &pb.SignupDriverRequest{
		Email: "existing@test.com",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %v", grpcCode(err))
	}
}

func TestDriverSignup_GetByEmailError(t *testing.T) {
	svc := &DriverService{
		repo: &driverRepoStub{
			getByEmailFn: func(ctx context.Context, email string) (*models.DriverModel, error) {
				return nil, status.Error(codes.Internal, "db error")
			},
		},
	}

	_, err := svc.Signup(context.Background(), &pb.SignupDriverRequest{
		Email: "newdriver@test.com",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", grpcCode(err))
	}
}

func TestDriverSignup_CreateError(t *testing.T) {
	svc := &DriverService{
		repo: &driverRepoStub{
			getByEmailFn: func(ctx context.Context, email string) (*models.DriverModel, error) {
				return nil, nil
			},
			createFn: func(ctx context.Context, req *pb.SignupDriverRequest) (*models.DriverModel, error) {
				return nil, status.Error(codes.Internal, "db error")
			},
		},
	}

	_, err := svc.Signup(context.Background(), &pb.SignupDriverRequest{
		Email: "newdriver@test.com",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if grpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", grpcCode(err))
	}
}

func TestDriverServiceInterface(t *testing.T) {
	var _ pb.DriverServiceServer = (*DriverService)(nil)
}
