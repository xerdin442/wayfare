package types

import "time"

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
	ID               string    `json:"id"`
	PackageSlug      string    `json:"packageSlug"`
	BasePrice        float64   `json:"basePrice"`
	TotalPriceInKobo *float64  `json:"totalPriceInKobo,omitempty"`
	ExpiresAt        time.Time `json:"expiresAt"`
	Route            Route     `json:"route"`
}

type Trip struct {
	ID           string   `json:"id"`
	UserID       string   `json:"userID"`
	Status       string   `json:"status"`
	SelectedFare RideFare `json:"selectedFare"`
	Route        Route    `json:"route"`
	Driver       *Driver  `json:"driver,omitempty"`
}

type Driver struct {
	ID             string     `json:"id"`
	Location       Coordinate `json:"location"`
	Geohash        string     `json:"geohash"`
	Name           string     `json:"name"`
	ProfilePicture string     `json:"profilePicture"`
	CarPlate       string     `json:"carPlate"`
}
