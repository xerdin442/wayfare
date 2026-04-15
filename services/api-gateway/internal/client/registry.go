package client

import (
	"github.com/rs/zerolog/log"
	rpc "github.com/xerdin442/wayfare/shared/pkg"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Registry struct {
	Trip   rpc.TripServiceClient
	Driver rpc.DriverServiceClient
	Rider  rpc.RiderServiceClient
	conns  []*grpc.ClientConn
}

func NewRegistry() *Registry {
	credentials := grpc.WithTransportCredentials(insecure.NewCredentials())

	tripConn, err := grpc.NewClient("trip-service:80", credentials)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Trip service")
		return nil
	}

	driverConn, err := grpc.NewClient("driver-service:80", credentials)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Driver service")
		return nil
	}

	riderConn, err := grpc.NewClient("rider-service:80", credentials)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Rider service")
		return nil
	}

	return &Registry{
		Trip:   rpc.NewTripServiceClient(tripConn),
		Driver: rpc.NewDriverServiceClient(driverConn),
		Rider:  rpc.NewRiderServiceClient(riderConn),
		conns:  []*grpc.ClientConn{tripConn, driverConn, riderConn},
	}
}

func (r *Registry) Close() {
	for _, conn := range r.conns {
		conn.Close()
	}
}
