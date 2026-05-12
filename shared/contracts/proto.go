package contracts

import (
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/types"
)

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

func MapRideFares(resp []*pb.RideFare) []types.RideFare {
	rideFares := make([]types.RideFare, len(resp))

	for _, fare := range resp {
		var routeGeometry []*types.Geometry
		for _, geometry := range fare.Route.Geometry {
			coords := make([]*types.Coordinate, len(geometry.Coordinates))
			for _, coord := range geometry.Coordinates {
				coords = append(coords, &types.Coordinate{
					Latitude:  coord.Latitude,
					Longitude: coord.Longitude,
				})
			}

			routeGeometry = append(routeGeometry, &types.Geometry{
				Coordinates: coords,
			})
		}

		route := types.Route{
			Distance: fare.Route.Distance,
			Duration: fare.Route.Duration,
			Geometry: routeGeometry,
		}

		rideFares = append(rideFares, types.RideFare{
			ID:          fare.Id,
			PackageSlug: types.CarPackage(fare.PackageSlug),
			Amount:      fare.Amount,
			Route:       route,
		})
	}

	return rideFares
}
