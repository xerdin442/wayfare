package contracts

import pb "github.com/xerdin442/wayfare/shared/pkg"

func (o *OsrmApiResponse) ToProto() *pb.Route {
	route := o.Routes[0]
	geometry := route.Geometry.Coordinates
	coordinates := make([]*pb.Coordinate, len(geometry))

	for i, coord := range geometry {
		coordinates[i] = &pb.Coordinate{
			Longitude: coord[0],
			Latitude:  coord[1],
		}
	}

	return &pb.Route{
		Distance: route.Distance,
		Duration: route.Duration,
		Geometry: []*pb.Geometry{
			{Coordinates: coordinates},
		},
	}
}

func (r *PreviewTripRequest) ToProto() *pb.PreviewTripRequest {
	return &pb.PreviewTripRequest{
		Pickup: &pb.Coordinate{
			Latitude:  r.Pickup.Latitude,
			Longitude: r.Pickup.Longitude,
		},
		Destination: &pb.Coordinate{
			Latitude:  r.Destination.Latitude,
			Longitude: r.Destination.Longitude,
		},
	}
}
