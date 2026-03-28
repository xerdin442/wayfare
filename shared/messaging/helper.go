package messaging

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/xerdin442/wayfare/shared/contracts"
)

type AmqpEventHandler func(ctx context.Context, body []byte) error

type MessageBus interface {
	PublishMessage(ctx context.Context, exchange AmqpExchange, routingKey contracts.AmqpEvent, msg contracts.AmqpMessage) error
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
	GatewayQueue             AmqpQueue = "gateway_queue"
	FindAndAssignDriverQueue AmqpQueue = "find_and_assign_driver_queue"
	DeadLetterQueue          AmqpQueue = "dead_letter_queue"
)
