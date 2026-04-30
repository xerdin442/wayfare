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
	DriverUpdateQueue AmqpQueue = "driver_update_queue"
	TripUpdateQueue   AmqpQueue = "trip_update_queue"
	PaymentQueue      AmqpQueue = "payment_queue"
	DeadLetterQueue   AmqpQueue = "dead_letter_queue"
)

type DriverUpdateQueuePayload struct {
	DriverID        string `json:"driver_id"`
	TripCountUpdate bool   `json:"trip_count_update,omitempty"`
}

type AssignDriverQueuePayload struct {
	Trip     types.Trip `json:"trip"`
	DriverID string     `json:"driver_id,omitempty"`
}

type TripUpdateQueuePayload struct {
	TripID       string `json:"trip_id"`
	DriverID     string `json:"driver_id,omitempty"`
	Rating       int64  `json:"rating,omitempty"`
	RiderComment string `json:"rider_comment,omitempty"`
}

type CashPaymentPayload struct {
	TripID       string `json:"trip_id"`
	RiderID      string `json:"rider_id"`
	Amount       int64  `json:"amount"`
	TripRating   int64  `json:"trip_rating"`
	RiderComment string `json:"rider_comment,omitempty"`
}

type CheckoutPaymentPayload struct {
	RiderID  string                `json:"rider_id"`
	Provider types.PaymentProvider `json:"provider"`
	Data     any                   `json:"data"`
}

type AmqpEvent string

const (
	// Trip events
	TripEventCreated             AmqpEvent = "trip.event.created"
	TripEventDriverAssigned      AmqpEvent = "trip.event.driver_assigned"
	TripEventDriverArrival       AmqpEvent = "trip.event.driver_arrival"
	TripEventNoDriversFound      AmqpEvent = "trip.event.no_drivers_found"
	TripEventDriverNotInterested AmqpEvent = "trip.event.driver_not_interested"
	TripEventDriverNotAvailable  AmqpEvent = "trip.event.driver_not_available"
	TripEventPaymentRequired     AmqpEvent = "trip.event.payment_required"

	// Trip commands
	TripCmdCompleted AmqpEvent = "trip.cmd.completed"
	TripCmdCancelled AmqpEvent = "trip.cmd.cancelled"
	TripCmdAborted   AmqpEvent = "trip.cmd.aborted"

	// Driver events
	DriverEventTripRequest AmqpEvent = "driver.event.trip_request"

	// Driver commands
	DriverCmdTripPickup      AmqpEvent = "driver.cmd.confirm_pickup"
	DriverCmdTripAccept      AmqpEvent = "driver.cmd.trip_accept"
	DriverCmdTripDecline     AmqpEvent = "driver.cmd.trip_decline"
	DriverCmdLocationUpdate  AmqpEvent = "driver.cmd.location_update"
	DriverCmdEndTrip         AmqpEvent = "driver.cmd.end_trip"
	DriverCmdTripCountUpdate AmqpEvent = "driver.cmd.trip_count_update"

	// Payment events
	PaymentEventWebhookReceived     AmqpEvent = "payment.event.webhook_received"
	PaymentEventSuccess             AmqpEvent = "payment.event.success"
	PaymentEventFailed              AmqpEvent = "payment.event.failed"
	PaymentEventCashOptionPreferred AmqpEvent = "payment.event.cash_option_preferred"
	PaymentEventCashReceived        AmqpEvent = "payment.event.cash_payment_received"
)
