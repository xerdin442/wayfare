package service

import (
	"context"

	"github.com/xerdin442/wayfare/shared/models"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/util"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type riderRepository interface {
	GetRiderByID(ctx context.Context, riderId string) (*models.RiderModel, error)
	GetRiderByEmail(ctx context.Context, email string) (*models.RiderModel, error)
	CreateRiderAccount(ctx context.Context, details *pb.SignupRiderRequest) (*models.RiderModel, error)
}

type RiderService struct {
	pb.UnimplementedRiderServiceServer
	repo riderRepository
}

func NewRiderService(r riderRepository) *RiderService {
	return &RiderService{
		repo: r,
	}
}

func (s *RiderService) GetRiderProfile(ctx context.Context, req *pb.GetProfileRequest) (*pb.RiderProfileResponse, error) {
	rider, err := s.repo.GetRiderByID(ctx, req.UserId)
	if err != nil {
		if err == util.ErrDocumentNotFound {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}

	return &pb.RiderProfileResponse{
		Rider: &pb.Rider{
			Id:             rider.ID.Hex(),
			Name:           rider.Name,
			Email:          rider.Email,
			Phone:          rider.Phone,
			ProfilePicture: rider.ProfilePicture,
		},
	}, nil
}

func (s *RiderService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.AuthResponse, error) {
	existingRider, err := s.repo.GetRiderByEmail(ctx, req.Email)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal server error")
	}

	if existingRider == nil {
		return nil, status.Error(codes.NotFound, "invalid email address")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(existingRider.Password), []byte(req.Password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "incorrect password")
	}

	return &pb.AuthResponse{
		UserId: existingRider.ID.Hex(),
	}, nil
}

func (s *RiderService) Signup(ctx context.Context, req *pb.SignupRiderRequest) (*pb.AuthResponse, error) {
	existingRider, err := s.repo.GetRiderByEmail(ctx, req.Email)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal server error")
	}

	if existingRider != nil {
		return nil, status.Error(codes.AlreadyExists, "Rider already exists with this email")
	}

	rider, err := s.repo.CreateRiderAccount(ctx, req)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal server error")
	}

	return &pb.AuthResponse{
		UserId: rider.ID.Hex(),
	}, nil
}
