package messaging

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/models"
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
	AnalyticsExchange  AmqpExchange = "analytics"
	DeadLetterExchange AmqpExchange = "dlx"
)

type AmqpQueue string

const (
	GatewayQueue      AmqpQueue = "gateway_queue"
	AssignDriverQueue AmqpQueue = "assign_driver_queue"
	DriverUpdateQueue AmqpQueue = "driver_update_queue"
	TripUpdateQueue   AmqpQueue = "trip_update_queue"
	PaymentQueue      AmqpQueue = "payment_queue"
	AnalyticsQueue    AmqpQueue = "analytics_queue"
	DeadLetterQueue   AmqpQueue = "dead_letter_queue"
)

type DriverUpdateQueuePayload struct {
	DriverID                string             `json:"driver_id,omitempty"`
	Status                  types.DriverStatus `json:"status,omitempty"`
	RecipientCode           string             `json:"recipient_code,omitempty"`
	TripCountUpdate         bool               `json:"trip_count_update,omitempty"`
	RideFare                int64              `json:"ride_fare,omitempty"`
	Tip                     int64              `json:"tip,omitempty"`
	BalanceUpdate           bool               `json:"balance_update,omitempty"`
	PendingReturnsUpdate    bool               `json:"pending_returns_update,omitempty"`
	OutstandingReturnsReset bool               `json:"outstanding_returns_reset,omitempty"`
}

type AssignDriverQueuePayload struct {
	Trip     types.Trip       `json:"trip"`
	Pickup   types.Coordinate `json:"pickup"`
	DriverID string           `json:"driver_id,omitempty"`
}

type TripUpdateQueuePayload struct {
	TripID       string    `json:"trip_id"`
	DriverID     string    `json:"driver_id,omitempty"`
	StartedAt    time.Time `json:"started_at,omitempty"`
	EndedAt      time.Time `json:"ended_at,omitempty"`
	Rating       int64     `json:"rating,omitempty"`
	RiderComment string    `json:"rider_comment,omitempty"`
	DriverTip    int64     `json:"driver_tip,omitempty"`
	CashPayment  bool      `json:"cash_payment,omitempty"`
}

type CashPaymentPayload struct {
	TripID  string `json:"trip_id"`
	RiderID string `json:"rider_id"`
	Amount  int64  `json:"amount"`
}

type PaymentWebhookPayload struct {
	Provider           types.PaymentProvider                `json:"provider"`
	PaystackWebhook    *contracts.PaystackWebhookPayload    `json:"paystack_webhook,omitempty"`
	FlutterwaveWebhook *contracts.FlutterwaveWebhookPayload `json:"flutterwave_webhook,omitempty"`
}

type DriverPayoutPayload struct {
	Drivers []*models.DriverModel `json:"drivers"`
}

type AnalyticsQueuePayload struct {
	Event *models.TripEventModel `json:"event"`
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
	TripEventRatingRequired      AmqpEvent = "trip.event.rating_required"

	// Trip commands
	TripCmdCompleted AmqpEvent = "trip.cmd.completed"
	TripCmdCancelled AmqpEvent = "trip.cmd.cancelled"
	TripCmdAborted   AmqpEvent = "trip.cmd.aborted"
	TripCmdStarted   AmqpEvent = "trip.cmd.started"
	TripCmdRated     AmqpEvent = "trip.cmd.rated"

	// Driver events
	DriverEventTripRequest AmqpEvent = "driver.event.trip_request"

	// Driver commands
	DriverCmdTripPickup     AmqpEvent = "driver.cmd.confirm_pickup"
	DriverCmdTripAccept     AmqpEvent = "driver.cmd.trip_accept"
	DriverCmdTripDecline    AmqpEvent = "driver.cmd.trip_decline"
	DriverCmdLocationUpdate AmqpEvent = "driver.cmd.location_update"
	DriverCmdStartTrip      AmqpEvent = "driver.cmd.start_trip"
	DriverCmdEndTrip        AmqpEvent = "driver.cmd.end_trip"
	DriverCmdDetailsUpdate  AmqpEvent = "driver.cmd.details_update"

	// Payment events
	PaymentEventWebhookReceived     AmqpEvent = "payment.event.webhook_received"
	PaymentEventSuccess             AmqpEvent = "payment.event.success"
	PaymentEventFailed              AmqpEvent = "payment.event.failed"
	PaymentEventCashOptionPreferred AmqpEvent = "payment.event.cash_option_preferred"
	PaymentEventCashReceived        AmqpEvent = "payment.event.cash_payment_received"

	// Payment commands
	PaymentCmdDriverPayout AmqpEvent = "payment.cmd.driver_payout"

	// Analytics events
	AnalyticsEventUpdate AmqpEvent = "analytics.event.update"
)
