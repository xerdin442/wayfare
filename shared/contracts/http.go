package contracts

import (
	"mime/multipart"

	"github.com/xerdin442/wayfare/shared/types"
)

type APIResponse struct {
	Data any `json:"data,omitempty"`
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

type OpenweatherApiResponse struct {
	Coord struct {
		Lon float64 `json:"lon"`
		Lat float64 `json:"lat"`
	} `json:"coord"`
	Weather []struct {
		ID          int    `json:"id"`
		Main        string `json:"main"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	} `json:"weather"`
	Visibility int `json:"visibility"`
	Rain       struct {
		OneH float64 `json:"1h"`
	} `json:"rain"`
}

type SignupDetails struct {
	Email    string `form:"email" binding:"required,email"`
	Password string `form:"password" binding:"required,min=8"`
	Name     string `form:"name" binding:"required"`
	Phone    string `form:"phone" binding:"required"`
}

type SignupDriverRequest struct {
	SignupDetails
	ProfileImage       *multipart.FileHeader   `form:"profileImage" binding:"required"`
	VerificationPhotos []*multipart.FileHeader `form:"verificationPhotos" binding:"required"`
	CarModel           string                  `form:"carModel" binding:"required"`
	CarColor           string                  `form:"carColor" binding:"required"`
	CarPlate           string                  `form:"carPlate" binding:"required"`
	AccountNumber      string                  `form:"accountNumber" binding:"required"`
	AccountName        string                  `form:"accountName" binding:"required"`
	BankName           string                  `form:"bankName" binding:"required"`
}

type SignupRiderRequest struct {
	SignupDetails
	ProfileImage *multipart.FileHeader `form:"profileImage"`
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
	RideFareId string `json:"rideFareId" binding:"required"`
}

type InitiatePaymentRequest struct {
	Email        string `json:"email" binding:"required,email"`
	TripRating   int64  `json:"tripRating" binding:"required,min=1,max=5"`
	RiderComment string `json:"riderComment,omitempty"`
	DriverTip    int64  `json:"driverTip,omitempty"` // Naira amount
}
