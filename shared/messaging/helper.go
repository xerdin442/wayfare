package messaging

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/xerdin442/wayfare/shared/types"
)

type AmqpMessage struct {
	Data []byte `json:"data"`
}

type AmqpDeliveryPayload struct {
	RoutingKey string `json:"routing_key"`
	Body       []byte `json:"body"`
}

type AmqpEventHandler func(ctx context.Context, p AmqpDeliveryPayload) error

type MessageBus interface {
	PublishMessage(ctx context.Context, exchange AmqpExchange, routingKey AmqpEvent, msg AmqpMessage) error
	ConsumeMessages(queueName AmqpQueue, handler func(context.Context, amqp.Delivery) error) error
}

type AmqpExchange string

const (
	GatewayExchange    AmqpExchange = "gateway"
	ServicesExchange   AmqpExchange = "services"
	DeadLetterExchange AmqpExchange = "dlx"
)

type AmqpQueue string

const (
	GatewayQueue      AmqpQueue = "gateway_queue"
	AssignDriverQueue AmqpQueue = "assign_driver_queue"
	TripUpdateQueue   AmqpQueue = "trip_update_queue"
	DeadLetterQueue   AmqpQueue = "dead_letter_queue"
)

type AssignDriverQueuePayload struct {
	Trip     types.Trip `json:"trip"`
	DriverID string     `json:"driver_id,omitempty"`
}

type TripUpdateQueuePayload struct {
	TripID string `json:"trip_id"`
}

type AmqpEvent string

const (
	// Trip events
	TripEventCreated             AmqpEvent = "trip.event.created"
	TripEventDriverAssigned      AmqpEvent = "trip.event.driver_assigned"
	TripEventNoDriversFound      AmqpEvent = "trip.event.no_drivers_found"
	TripEventDriverNotInterested AmqpEvent = "trip.event.driver_not_interested"
	TripEventDriverNotAvailable  AmqpEvent = "trip.event.driver_not_available"

	// Trip commands
	TripCmdCompleted AmqpEvent = "trip.cmd.completed"
	TripCmdCancelled AmqpEvent = "trip.cmd.cancelled"
	TripCmdAborted   AmqpEvent = "trip.cmd.aborted"

	// Driver events
	DriverEventTripRequest AmqpEvent = "driver.event.trip_request"

	// Driver commands
	DriverCmdTripPickup     AmqpEvent = "driver.cmd.confirm_pickup"
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
