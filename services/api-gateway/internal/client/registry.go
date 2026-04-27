package client

import (
	"github.com/rs/zerolog/log"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/tracing"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Registry struct {
	Trip    pb.TripServiceClient
	Driver  pb.DriverServiceClient
	Rider   pb.RiderServiceClient
	Payment pb.PaymentServiceClient
	conns   []*grpc.ClientConn
}

func NewRegistry() *Registry {
	dialOptions := tracing.DialOptionsWithTracing()
	dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))

	tripConn, err := grpc.NewClient("trip-service:80", dialOptions...)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Trip service")
		return nil
	}

	driverConn, err := grpc.NewClient("driver-service:80", dialOptions...)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Driver service")
		return nil
	}

	riderConn, err := grpc.NewClient("rider-service:80", dialOptions...)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Rider service")
		return nil
	}

	paymentConn, err := grpc.NewClient("payment-service:80", dialOptions...)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Payment service")
		return nil
	}

	return &Registry{
		Trip:    pb.NewTripServiceClient(tripConn),
		Driver:  pb.NewDriverServiceClient(driverConn),
		Rider:   pb.NewRiderServiceClient(riderConn),
		Payment: pb.NewPaymentServiceClient(paymentConn),
		conns:   []*grpc.ClientConn{tripConn, driverConn, riderConn, paymentConn},
	}
}

func (r *Registry) Close() {
	for _, conn := range r.conns {
		conn.Close()
	}
}
