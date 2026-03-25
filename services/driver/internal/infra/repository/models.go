package repo

import (
	"github.com/xerdin442/wayfare/shared/types"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type DriverModel struct {
	ID             bson.ObjectID    `bson:"_id,omitempty"`
	Name           string           `bson:"name"`
	ProfilePicture string           `bson:"profile_picture"`
	CarPackage     types.CarPackage `bson:"car_package,omitempty"`
	CarPlate       string           `bson:"car_plate"`
}
