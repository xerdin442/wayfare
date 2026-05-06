package util

import "errors"

var (
	ErrMissingRoleHeader   = errors.New("missing or invalid x-user-role header")
	ErrDocumentNotFound    = errors.New("no document found")
	ErrTripSessionExpired  = errors.New("trip session has expired. please select another route on the map")
	ErrUnsupportedLocation = errors.New("wayfare is not yet available in this location")
	ErrGatewayUnavailable  = errors.New("payment gateway is currently unavailable")
	ErrApiRequestFailure   = errors.New("failed to send api request")
	ErrUnsupportedBank     = errors.New("wayfare does not support payouts to this bank")
	ErrAccountNameMismatch = errors.New("account name does not match the name registered with your bank. please check the spelling or order of your account name")
	ErrUnsupportedFileType = errors.New("unsupported file type")
)
