package types

type UserRole string

const (
	RoleRider  UserRole = "rider"
	RoleDriver UserRole = "driver"
)

type CarPackage string

const (
	PackageLuxury CarPackage = "luxury"
	PackageSedan  CarPackage = "sedan"
	PackageSUV    CarPackage = "suv"
)

type TripStatus string

const (
	TripStatusSearching       TripStatus = "searching"
	TripStatusAborted         TripStatus = "aborted"
	TripStatusMatched         TripStatus = "matched"
	TripStatusActive          TripStatus = "active"
	TripStatusAwaitingPayment TripStatus = "awaiting_payment"
	TripStatusCompleted       TripStatus = "completed"
	TripStatusCancelled       TripStatus = "cancelled"
)

type PaymentStatus string

const (
	PaymentStatusPending  PaymentStatus = "pending"
	PaymentStatusSuccess  PaymentStatus = "success"
	PaymentStatusFailed   PaymentStatus = "failed"
	PaymentStatusReversed PaymentStatus = "reversed"
	PaymentStatusAborted  PaymentStatus = "aborted"
)

type PaymentProvider string

const (
	ProviderPaystack    PaymentProvider = "paystack"
	ProviderFlutterwave PaymentProvider = "flutterwave"
)

type TransactionType string

const (
	TransactionCheckout TransactionType = "checkout"
	TransactionPayout   TransactionType = "payout"
)

type DriverTier string

const (
	TierBronze DriverTier = "bronze"
	TierSilver DriverTier = "silver"
	TierGold   DriverTier = "gold"
)

type DriverStatus string

const (
	DriverStatusOnline  DriverStatus = "online"
	DriverStatusOffline DriverStatus = "offline"
	DriverStatusBusy    DriverStatus = "busy"
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
	ID          string     `json:"id"`
	PackageSlug CarPackage `json:"packageSlug"`
	Amount      int64      `json:"amount"`
	Route       Route      `json:"route"`
}

type Trip struct {
	ID           string     `json:"id"`
	UserID       string     `json:"userId"`
	DriverID     string     `json:"driverId,omitempty"`
	Status       TripStatus `json:"status"`
	SelectedFare RideFare   `json:"selectedFare"`
}

type Driver struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	Email               string     `json:"email"`
	ProfilePicture      string     `json:"profilePicture"`
	CarPlate            string     `json:"carPlate"`
	CurrentRating       float64    `json:"currentRating"`
	TotalCompletedTrips int64      `json:"totalCompletedTrips"`
	Tier                DriverTier `json:"tier"`
}

type Rider struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	ProfilePicture string `json:"profilePicture"`
}
