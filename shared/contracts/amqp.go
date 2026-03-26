package contracts

type AmqpMessage struct {
	OwnerID string `json:"ownerId"`
	Data    []byte `json:"data"`
}

type AmqpEvent string

const (
	// Trip events
	TripEventCreated             AmqpEvent = "trip.event.created"
	TripEventDriverAssigned      AmqpEvent = "trip.event.driver_assigned"
	TripEventNoDriversFound      AmqpEvent = "trip.event.no_drivers_found"
	TripEventDriverNotInterested AmqpEvent = "trip.event.driver_not_interested"

	// Driver commands
	DriverCmdTripRequest AmqpEvent = "driver.cmd.trip_request"
	DriverCmdTripAccept  AmqpEvent = "driver.cmd.trip_accept"
	DriverCmdTripDecline AmqpEvent = "driver.cmd.trip_decline"
	DriverCmdLocation    AmqpEvent = "driver.cmd.location"
	DriverCmdRegister    AmqpEvent = "driver.cmd.register"

	// Payment events
	PaymentEventSessionCreated AmqpEvent = "payment.event.session_created"
	PaymentEventSuccess        AmqpEvent = "payment.event.success"
	PaymentEventFailed         AmqpEvent = "payment.event.failed"
	PaymentEventCancelled      AmqpEvent = "payment.event.cancelled"

	// Payment commands
	PaymentCmdCreateSession AmqpEvent = "payment.cmd.create_session"
)
