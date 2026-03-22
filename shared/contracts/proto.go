package contracts

import rpc "github.com/xerdin442/wayfare/shared/pkg"

func (o *OsrmApiResponse) ToProto() *rpc.Route {
	route := o.Routes[0]
	geometry := route.Geometry.Coordinates
	coordinates := make([]*rpc.Coordinate, len(geometry))

	for i, coord := range geometry {
		coordinates[i] = &rpc.Coordinate{
			Longitude: coord[0],
			Latitude:  coord[1],
		}
	}

	return &rpc.Route{
		Distance: route.Distance,
		Duration: route.Duration,
		Geometry: []*rpc.Geometry{
			{Coordinates: coordinates},
		},
	}
}

func (r *PreviewTripRequest) ToProto() *rpc.PreviewTripRequest {
	return &rpc.PreviewTripRequest{
		Pickup: &rpc.Coordinate{
			Latitude:  r.Pickup.Latitude,
			Longitude: r.Pickup.Longitude,
		},
		Destination: &rpc.Coordinate{
			Latitude:  r.Destination.Latitude,
			Longitude: r.Destination.Longitude,
		},
	}
}
