package service

import (
	"context"
	"strings"

	repo "github.com/xerdin442/wayfare/services/rider/internal/infra/repository"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RiderService struct {
	pb.UnimplementedRiderServiceServer
	repo *repo.RiderRepository
}

func NewRiderService(r *repo.RiderRepository) *RiderService {
	return &RiderService{
		repo: r,
	}
}

func (s *RiderService) GetRiderProfile(ctx context.Context, req *pb.GetProfileRequest) (*pb.RiderProfileResponse, error) {
	rider, err := s.repo.GetRiderByID(ctx, req.UserId)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, "Rider not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.RiderProfileResponse{
		Rider: &pb.Rider{
			Id:             rider.ID.Hex(),
			Name:           rider.Name,
			Email:          rider.Email,
			ProfilePicture: rider.ProfilePicture,
		},
	}, nil
}

func (s *RiderService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.AuthResponse, error) {
	rider, err := s.repo.GetRiderByEmail(ctx, req.Email)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, "Invalid email address")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err := bcrypt.CompareHashAndPassword([]byte(rider.Password), []byte(req.Password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "Incorrect password")
	}

	return &pb.AuthResponse{
		UserId: rider.ID.Hex(),
	}, nil
}

func (s *RiderService) Signup(ctx context.Context, req *pb.SignupRiderRequest) (*pb.AuthResponse, error) {
	_, err := s.repo.GetRiderByEmail(ctx, req.Email)
	if err == nil {
		return nil, status.Error(codes.AlreadyExists, "Rider already exists with this email")
	}

	rider, err := s.repo.CreateRiderAccount(ctx, req)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.AuthResponse{
		UserId: rider.ID.Hex(),
	}, nil
}
