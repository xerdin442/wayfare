package contracts

type AmqpMessage struct {
	OwnerID string `json:"owner_id"`
	Data    []byte `json:"data"`
}

type AmqpEvent string

const (
	// Client response
	GatewayEventWsResponse AmqpEvent = "user.*" // user.{userID}

	// Trip events
	TripEventCreated             AmqpEvent = "trip.event.created"
	TripEventDriverAssigned      AmqpEvent = "trip.event.driver_assigned"
	TripEventNoDriversFound      AmqpEvent = "trip.event.no_drivers_found"
	TripEventDriverNotInterested AmqpEvent = "trip.event.driver_not_interested"
	TripEventDriverNotAvailable  AmqpEvent = "trip.event.driver_not_available"

	// Trip commands
	TripCmdCompleted AmqpEvent = "trip.cmd.completed"
	TripCmdCancelled AmqpEvent = "trip.cmd.cancelled"

	// Driver events
	DriverEventTripRequest AmqpEvent = "driver.event.trip_request"

	// Driver commands
	DriverCmdTripAccept     AmqpEvent = "driver.cmd.trip_accept"
	DriverCmdTripDecline    AmqpEvent = "driver.cmd.trip_decline"
	DriverCmdLocationUpdate AmqpEvent = "driver.cmd.location_update"

	// Payment events
	PaymentEventSessionCreated AmqpEvent = "payment.event.session_created"
	PaymentEventSuccess        AmqpEvent = "payment.event.success"
	PaymentEventFailed         AmqpEvent = "payment.event.failed"
	PaymentEventCancelled      AmqpEvent = "payment.event.cancelled"

	// Payment commands
	PaymentCmdCreateSession AmqpEvent = "payment.cmd.create_session"
)
