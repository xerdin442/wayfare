package handlers

import "github.com/xerdin442/wayfare/shared/types"

type PreviewTripRequest struct {
	Pickup      types.Coordinate `json:"pickup"`
	Destination types.Coordinate `json:"destination"`
}
