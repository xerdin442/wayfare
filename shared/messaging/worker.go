package messaging

import (
	"context"
	"fmt"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type EventWorker struct {
	bus      MessageBus
	queue    AmqpQueue
	mu       sync.RWMutex
	handlers map[AmqpEvent]AmqpEventHandler
}

func NewEventWorker(bus MessageBus, queue AmqpQueue) *EventWorker {
	return &EventWorker{
		bus:      bus,
		queue:    queue,
		handlers: make(map[AmqpEvent]AmqpEventHandler),
	}
}

func (c *EventWorker) RegisterHandler(h AmqpEventHandler, events ...AmqpEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, e := range events {
		c.handlers[e] = h
	}
}

func (c *EventWorker) Start(ctx context.Context) error {
	return c.bus.ConsumeMessages(c.queue, func(ctx context.Context, msg amqp.Delivery) error {
		event := AmqpEvent(msg.RoutingKey)
		handler, ok := c.handlers[event]
		if !ok {
			return fmt.Errorf("No handler registered for %s event", event)
		}

		return handler(ctx, AmqpDeliveryPayload{
			RoutingKey: msg.RoutingKey,
			Body:       msg.Body,
		})
	})
}
