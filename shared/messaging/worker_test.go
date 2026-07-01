package messaging

import (
	"context"
	"fmt"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

type stubBus struct {
	queueName AmqpQueue
	handler   func(context.Context, amqp.Delivery) error
}

func (s *stubBus) PublishMessage(context.Context, AmqpExchange, AmqpEvent, AmqpMessage) error {
	return nil
}

func (s *stubBus) ConsumeMessages(queueName AmqpQueue, handler func(context.Context, amqp.Delivery) error) error {
	s.queueName = queueName
	s.handler = handler
	return nil
}

func TestEventWorkerInvokesRegisteredHandler(t *testing.T) {
	bus := &stubBus{}
	worker := NewEventWorker(bus, TripUpdateQueue)
	invoked := false

	worker.RegisterHandler(func(ctx context.Context, payload AmqpDeliveryPayload) error {
		invoked = true
		if payload.RoutingKey != string(TripEventCreated) {
			t.Fatalf("unexpected routing key %q", payload.RoutingKey)
		}
		if string(payload.Body) != "payload" {
			t.Fatalf("unexpected body %q", payload.Body)
		}
		return nil
	}, TripEventCreated)

	if err := worker.Start(); err != nil {
		t.Fatalf("expected worker to start cleanly, got %v", err)
	}

	if bus.queueName != TripUpdateQueue {
		t.Fatalf("expected queue %q, got %q", TripUpdateQueue, bus.queueName)
	}

	if err := bus.handler(context.Background(), amqp.Delivery{RoutingKey: string(TripEventCreated), Body: []byte("payload")}); err != nil {
		t.Fatalf("expected handler invocation to succeed, got %v", err)
	}

	if !invoked {
		t.Fatal("expected registered handler to be invoked")
	}
}

func TestEventWorkerReturnsErrorForUnhandledEvent(t *testing.T) {
	bus := &stubBus{}
	worker := NewEventWorker(bus, TripUpdateQueue)
	worker.RegisterHandler(func(context.Context, AmqpDeliveryPayload) error { return nil }, TripEventCreated)

	if err := worker.Start(); err != nil {
		t.Fatalf("expected worker to start cleanly, got %v", err)
	}

	err := bus.handler(context.Background(), amqp.Delivery{RoutingKey: string(TripEventDriverAssigned)})
	if err == nil {
		t.Fatal("expected unhandled event to return an error")
	}

	if got := err.Error(); got != fmt.Sprintf("No handler registered for %s event", TripEventDriverAssigned) {
		t.Fatalf("unexpected error %q", got)
	}
}
