package contracts

import (
	"mime/multipart"

	"github.com/xerdin442/wayfare/shared/types"
)

type APIResponse struct {
	Data  any       `json:"data,omitempty"`
	Error *APIError `json:"error,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type OsrmApiResponse struct {
	Routes []struct {
		Distance float64 `json:"distance"`
		Duration float64 `json:"duration"`
		Geometry struct {
			Coordinates [][]float64 `json:"coordinates"`
			Type        string      `json:"type"`
		} `json:"geometry"`
	} `json:"routes"`
}

type SignupDetails struct {
	Email    string `form:"email" binding:"required,email"`
	Password string `form:"password" binding:"required,min=8"`
	Name     string `form:"name" binding:"required"`
}

type SignupDriverRequest struct {
	SignupDetails
	ProfileImage *multipart.FileHeader `form:"profile_image" binding:"required"`
	CarPackage   string                `form:"car_package" binding:"required"`
	CarPlate     string                `form:"car_plate" binding:"required"`
}

type SignupRiderRequest struct {
	SignupDetails
	ProfileImage *multipart.FileHeader `form:"profile_image"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type PreviewTripRequest struct {
	Pickup      types.Coordinate `json:"pickup" binding:"required"`
	Destination types.Coordinate `json:"destination" binding:"required"`
}

type StartTripRequest struct {
	RideFareID string `json:"rideFareID" binding:"required"`
}
