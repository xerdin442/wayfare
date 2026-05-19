package contracts

import (
	"github.com/xerdin442/wayfare/shared/models"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/types"
)

func (o *OsrmApiResponse) ToProto() *pb.Route {
	route := o.Routes[0]
	geometry := route.Geometry.Coordinates
	coordinates := make([]*pb.Coordinate, 0, len(geometry))

	for _, coord := range geometry {
		coordinates = append(coordinates, &pb.Coordinate{
			Longitude: coord[0],
			Latitude:  coord[1],
		})
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

func MapPbRideFares(resp []*pb.RideFare) []types.RideFare {
	rideFares := make([]types.RideFare, 0, len(resp))

	for _, fare := range resp {
		var routeGeometry []*types.Geometry
		for _, geometry := range fare.Route.Geometry {
			coords := make([]*types.Coordinate, len(geometry.Coordinates))
			for _, coord := range geometry.Coordinates {
				coords = append(coords, &types.Coordinate{
					Latitude:  coord.Latitude,
					Longitude: coord.Longitude,
					Address:   coord.Address,
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

func MapTripModels(models []*models.TripModel) []*pb.Trip {
	if len(models) == 0 {
		return nil
	}

	trips := make([]*pb.Trip, 0, len(models))

	for _, trip := range models {
		pickupAddress := ""
		destinationAddress := ""
		if len(trip.Route.Addresses) >= 2 {
			pickupAddress = trip.Route.Addresses[0]
			destinationAddress = trip.Route.Addresses[1]
		}

		driverId := ""
		if !trip.DriverID.IsZero() {
			driverId = trip.DriverID.Hex()
		}

		trips = append(trips, &pb.Trip{
			Id:       trip.ID.Hex(),
			UserId:   trip.UserID.Hex(),
			DriverId: driverId,
			Status:   string(trip.Status),
			SelectedFare: &pb.RideFare{
				PackageSlug: string(trip.CarPackage),
				Amount:      trip.RideFare,
				Route: &pb.Route{
					Distance: trip.Route.Distance,
					Duration: trip.Route.Duration,
					Geometry: []*pb.Geometry{
						{
							Coordinates: []*pb.Coordinate{
								{
									Latitude:  trip.Route.Pickup.Coordinates.Lat(),
									Longitude: trip.Route.Pickup.Coordinates.Lon(),
									Address:   pickupAddress,
								},
								{
									Latitude:  trip.Route.Destination.Coordinates.Lat(),
									Longitude: trip.Route.Destination.Coordinates.Lon(),
									Address:   destinationAddress,
								},
							},
						},
					},
				},
			},
		})
	}

	return trips
}

func MapPbTrips(trips []*pb.Trip) []types.Trip {
	if len(trips) == 0 {
		return nil
	}

	result := make([]types.Trip, 0, len(trips))
	for _, trip := range trips {
		var routeGeometry []*types.Geometry
		for _, geometry := range trip.SelectedFare.Route.Geometry {
			coords := make([]*types.Coordinate, len(geometry.Coordinates))
			for _, coord := range geometry.Coordinates {
				coords = append(coords, &types.Coordinate{
					Latitude:  coord.Latitude,
					Longitude: coord.Longitude,
					Address:   coord.Address,
				})
			}
			routeGeometry = append(routeGeometry, &types.Geometry{
				Coordinates: coords,
			})
		}

		route := types.Route{
			Distance: trip.SelectedFare.Route.Distance,
			Duration: trip.SelectedFare.Route.Duration,
			Geometry: routeGeometry,
		}

		result = append(result, types.Trip{
			ID:       trip.Id,
			UserID:   trip.UserId,
			DriverID: trip.DriverId,
			Status:   types.TripStatus(trip.Status),
			SelectedFare: types.RideFare{
				ID:          trip.SelectedFare.Id,
				PackageSlug: types.CarPackage(trip.SelectedFare.PackageSlug),
				Amount:      trip.SelectedFare.Amount,
				Route:       route,
			},
		})
	}

	return result
}
