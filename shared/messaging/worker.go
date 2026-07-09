package messaging

import (
	"context"
	"fmt"
	"strings"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type EventWorker struct {
	bus      MessageBus
	queue    AmqpQueue
	mu       sync.RWMutex
	handlers map[AmqpEvent]AmqpEventHandler
	patterns []patternHandler
}

type patternHandler struct {
	pattern string
	handler AmqpEventHandler
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
		if strings.Contains(string(e), "*") {
			c.patterns = append(c.patterns, patternHandler{pattern: string(e), handler: h})
			continue
		}
		c.handlers[e] = h
	}
}

func (c *EventWorker) resolveHandler(routingKey string) (AmqpEventHandler, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if h, ok := c.handlers[AmqpEvent(routingKey)]; ok {
		return h, true
	}

	for _, ph := range c.patterns {
		if topicMatch(ph.pattern, routingKey) {
			return ph.handler, true
		}
	}

	return nil, false
}

func (c *EventWorker) Start() error {
	return c.bus.ConsumeMessages(c.queue, func(ctx context.Context, msg amqp.Delivery) error {
		handler, ok := c.resolveHandler(msg.RoutingKey)
		if !ok {
			return fmt.Errorf("No handler registered for %s event", msg.RoutingKey)
		}

		return handler(ctx, AmqpDeliveryPayload{
			RoutingKey: msg.RoutingKey,
			Body:       msg.Body,
		})
	})
}

func topicMatch(pattern, routingKey string) bool {
	pParts := strings.Split(pattern, ".")
	kParts := strings.Split(routingKey, ".")

	if len(pParts) != len(kParts) {
		return false
	}

	for i, p := range pParts {
		if p == "*" {
			continue
		}
		if p != kParts[i] {
			return false
		}
	}

	return true
}
