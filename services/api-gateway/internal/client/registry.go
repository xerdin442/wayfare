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

	return &Registry{
		Trip:   rpc.NewTripServiceClient(tripConn),
		Driver: rpc.NewDriverServiceClient(driverConn),
		conns:  []*grpc.ClientConn{tripConn},
	}
}

func (r *Registry) Close() {
	for _, conn := range r.conns {
		conn.Close()
	}
}
