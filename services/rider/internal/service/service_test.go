package service

import (
	"context"
	"testing"

	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/models"
	"github.com/xerdin442/wayfare/shared/util"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type riderRepoStub struct {
	getByIDFn    func(ctx context.Context, id string) (*models.RiderModel, error)
	getByEmailFn func(ctx context.Context, email string) (*models.RiderModel, error)
	createFn     func(ctx context.Context, req *pb.SignupRiderRequest) (*models.RiderModel, error)
}

func (s *riderRepoStub) GetRiderByID(ctx context.Context, id string) (*models.RiderModel, error) {
	return s.getByIDFn(ctx, id)
}

func (s *riderRepoStub) GetRiderByEmail(ctx context.Context, email string) (*models.RiderModel, error) {
	return s.getByEmailFn(ctx, email)
}

func (s *riderRepoStub) CreateRiderAccount(ctx context.Context, req *pb.SignupRiderRequest) (*models.RiderModel, error) {
	return s.createFn(ctx, req)
}

func validRiderModel() *models.RiderModel {
	return &models.RiderModel{
		ID:             bson.NewObjectID(),
		Name:           "Jane Rider",
		Email:          "rider@test.com",
		Phone:          "08098765432",
		Password:       hashPw("password123"),
		ProfilePicture: "https://example.com/rider.jpg",
	}
}

func hashPw(pw string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return string(h)
}

func riderGrpcCode(err error) codes.Code {
	st, ok := status.FromError(err)
	if !ok {
		return codes.OK
	}
	return st.Code()
}

func TestRiderGetProfile_Success(t *testing.T) {
	r := validRiderModel()

	svc := &RiderService{
		repo: &riderRepoStub{
			getByIDFn: func(ctx context.Context, id string) (*models.RiderModel, error) {
				if id != r.ID.Hex() {
					t.Fatalf("unexpected id: %s", id)
				}
				return r, nil
			},
		},
	}

	resp, err := svc.GetRiderProfile(context.Background(), &pb.GetProfileRequest{
		UserId: r.ID.Hex(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rd := resp.Rider
	if rd.Name != "Jane Rider" {
		t.Fatalf("expected Jane Rider, got %s", rd.Name)
	}
	if rd.Email != "rider@test.com" {
		t.Fatalf("expected rider@test.com, got %s", rd.Email)
	}
	if rd.Phone != "08098765432" {
		t.Fatalf("expected phone, got %s", rd.Phone)
	}
	if rd.ProfilePicture != "https://example.com/rider.jpg" {
		t.Fatalf("expected profile picture url")
	}
}

func TestRiderGetProfile_NotFound(t *testing.T) {
	svc := &RiderService{
		repo: &riderRepoStub{
			getByIDFn: func(ctx context.Context, id string) (*models.RiderModel, error) {
				return nil, util.ErrDocumentNotFound
			},
		},
	}

	_, err := svc.GetRiderProfile(context.Background(), &pb.GetProfileRequest{
		UserId: "507f1f77bcf86cd799439011",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if riderGrpcCode(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", riderGrpcCode(err))
	}
}

func TestRiderGetProfile_InternalError(t *testing.T) {
	svc := &RiderService{
		repo: &riderRepoStub{
			getByIDFn: func(ctx context.Context, id string) (*models.RiderModel, error) {
				return nil, status.Error(codes.Internal, "db error")
			},
		},
	}

	_, err := svc.GetRiderProfile(context.Background(), &pb.GetProfileRequest{
		UserId: "507f1f77bcf86cd799439011",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if riderGrpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", riderGrpcCode(err))
	}
}

func TestRiderLogin_Success(t *testing.T) {
	r := validRiderModel()

	svc := &RiderService{
		repo: &riderRepoStub{
			getByEmailFn: func(ctx context.Context, email string) (*models.RiderModel, error) {
				if email != "rider@test.com" {
					t.Fatalf("unexpected email: %s", email)
				}
				return r, nil
			},
		},
	}

	resp, err := svc.Login(context.Background(), &pb.LoginRequest{
		Email:    "rider@test.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.UserId != r.ID.Hex() {
		t.Fatalf("expected user id %s, got %s", r.ID.Hex(), resp.UserId)
	}
}

func TestRiderLogin_NotFound(t *testing.T) {
	svc := &RiderService{
		repo: &riderRepoStub{
			getByEmailFn: func(ctx context.Context, email string) (*models.RiderModel, error) {
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
	if riderGrpcCode(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", riderGrpcCode(err))
	}
}

func TestRiderLogin_WrongPassword(t *testing.T) {
	r := validRiderModel()

	svc := &RiderService{
		repo: &riderRepoStub{
			getByEmailFn: func(ctx context.Context, email string) (*models.RiderModel, error) {
				return r, nil
			},
		},
	}

	_, err := svc.Login(context.Background(), &pb.LoginRequest{
		Email:    "rider@test.com",
		Password: "wrongpassword",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if riderGrpcCode(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", riderGrpcCode(err))
	}
}

func TestRiderLogin_RepoError(t *testing.T) {
	svc := &RiderService{
		repo: &riderRepoStub{
			getByEmailFn: func(ctx context.Context, email string) (*models.RiderModel, error) {
				return nil, status.Error(codes.Internal, "db connection lost")
			},
		},
	}

	_, err := svc.Login(context.Background(), &pb.LoginRequest{
		Email:    "rider@test.com",
		Password: "password123",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if riderGrpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", riderGrpcCode(err))
	}
}

func TestRiderSignup_Success(t *testing.T) {
	created := validRiderModel()

	svc := &RiderService{
		repo: &riderRepoStub{
			getByEmailFn: func(ctx context.Context, email string) (*models.RiderModel, error) {
				return nil, nil
			},
			createFn: func(ctx context.Context, req *pb.SignupRiderRequest) (*models.RiderModel, error) {
				if req.Email != "newrider@test.com" {
					t.Fatalf("unexpected email: %s", req.Email)
				}
				return created, nil
			},
		},
	}

	resp, err := svc.Signup(context.Background(), &pb.SignupRiderRequest{
		Email: "newrider@test.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.UserId != created.ID.Hex() {
		t.Fatalf("expected user id %s, got %s", created.ID.Hex(), resp.UserId)
	}
}

func TestRiderSignup_AlreadyExists(t *testing.T) {
	svc := &RiderService{
		repo: &riderRepoStub{
			getByEmailFn: func(ctx context.Context, email string) (*models.RiderModel, error) {
				return validRiderModel(), nil
			},
		},
	}

	_, err := svc.Signup(context.Background(), &pb.SignupRiderRequest{
		Email: "existing@test.com",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if riderGrpcCode(err) != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %v", riderGrpcCode(err))
	}
}

func TestRiderSignup_GetByEmailError(t *testing.T) {
	svc := &RiderService{
		repo: &riderRepoStub{
			getByEmailFn: func(ctx context.Context, email string) (*models.RiderModel, error) {
				return nil, status.Error(codes.Internal, "db error")
			},
		},
	}

	_, err := svc.Signup(context.Background(), &pb.SignupRiderRequest{
		Email: "newrider@test.com",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if riderGrpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", riderGrpcCode(err))
	}
}

func TestRiderSignup_CreateError(t *testing.T) {
	svc := &RiderService{
		repo: &riderRepoStub{
			getByEmailFn: func(ctx context.Context, email string) (*models.RiderModel, error) {
				return nil, nil
			},
			createFn: func(ctx context.Context, req *pb.SignupRiderRequest) (*models.RiderModel, error) {
				return nil, status.Error(codes.Internal, "db error")
			},
		},
	}

	_, err := svc.Signup(context.Background(), &pb.SignupRiderRequest{
		Email: "newrider@test.com",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if riderGrpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", riderGrpcCode(err))
	}
}

func TestRiderServiceInterface(t *testing.T) {
	var _ pb.RiderServiceServer = (*RiderService)(nil)
}
