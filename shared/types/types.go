package types

type CarPackage string

const (
	PackageLuxury CarPackage = "luxury"
	PackageSedan  CarPackage = "sedan"
	PackageSUV    CarPackage = "suv"
)

type TripStatus string

const (
	TripStatusSearching TripStatus = "searching"
	TripStatusAborted   TripStatus = "aborted"
	TripStatusMatched   TripStatus = "matched"
	TripStatusActive    TripStatus = "active"
	TripStatusCompleted TripStatus = "completed"
	TripStatusCancelled TripStatus = "cancelled"
)

type Route struct {
	Distance float64     `json:"distance"`
	Duration float64     `json:"duration"`
	Geometry []*Geometry `json:"geometry"`
}

type Geometry struct {
	Coordinates []*Coordinate `json:"coordinates"`
}

type Coordinate struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type RideFare struct {
	ID               string     `json:"id"`
	PackageSlug      CarPackage `json:"packageSlug"`
	BasePrice        int64      `json:"basePrice"`
	TotalPriceInKobo int64      `json:"totalPriceInKobo,omitempty"`
	Route            Route      `json:"route"`
}

type Trip struct {
	ID           string     `json:"id"`
	UserID       string     `json:"userID"`
	DriverID     string     `json:"driverID,omitempty"`
	Status       TripStatus `json:"status"`
	SelectedFare RideFare   `json:"selectedFare"`
	Route        Route      `json:"route"`
}

type Driver struct {
	ID             string     `json:"id"`
	Location       Coordinate `json:"location"`
	Name           string     `json:"name"`
	ProfilePicture string     `json:"profilePicture"`
	CarPlate       string     `json:"carPlate"`
}

type Rider struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ProfilePicture string `json:"profilePicture"`
}

type PaymentSession struct {
	TripID    string `json:"tripID"`
	SessionID string `json:"sessionID"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
}
