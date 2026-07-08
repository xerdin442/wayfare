package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/rs/zerolog/log"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/tracing"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type Registry struct {
	Trip    pb.TripServiceClient
	Driver  pb.DriverServiceClient
	Rider   pb.RiderServiceClient
	Payment pb.PaymentServiceClient
	conns   []*grpc.ClientConn
}

func NewRegistry(servicePort int, isDev bool) *Registry {
	dialOptions := tracing.DialOptionsWithTracing()
	if isDev {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		// Load CA certificates
		systemRoots, err := x509.SystemCertPool()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to load CA certificates from system roots")
		}

		// Create TLS credentials
		creds := credentials.NewTLS(&tls.Config{RootCAs: systemRoots})
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(creds))
	}

	tripConn, err := grpc.NewClient(fmt.Sprintf("trip-service:%d", servicePort), dialOptions...)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Trip service")
		return nil
	}

	driverConn, err := grpc.NewClient(fmt.Sprintf("driver-service:%d", servicePort), dialOptions...)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Driver service")
		return nil
	}

	riderConn, err := grpc.NewClient(fmt.Sprintf("rider-service:%d", servicePort), dialOptions...)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Rider service")
		return nil
	}

	paymentConn, err := grpc.NewClient(fmt.Sprintf("payment-service:%d", servicePort), dialOptions...)
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
