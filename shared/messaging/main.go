package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/retry"
	"github.com/xerdin442/wayfare/shared/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type RabbitMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	tracer  trace.Tracer
}

func NewRabbitMQ(uri string) *RabbitMQ {
	_, err := amqp.ParseURI(uri)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid AMQP connection URI")
	}

	conn, err := amqp.Dial(uri)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to RabbitMQ")
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		log.Fatal().Err(err).Msg("Failed to create messaging channel")
	}

	rmq := &RabbitMQ{
		conn:    conn,
		channel: ch,
		tracer:  tracing.GetTracer("rabbitmq"),
	}

	if err := rmq.setupExchangesAndQueues(); err != nil {
		rmq.Close()
		log.Fatal().Err(err).Msg("Failed to setup exchanges and queues")
	}

	return rmq
}

func (r *RabbitMQ) ConsumeMessages(queueName AmqpQueue, handler func(context.Context, amqp.Delivery) error) error {
	// Ensure fair dispatch
	err := r.channel.Qos(
		1,     // prefetchCount
		0,     // prefetchSize
		false, // global
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to set QoS and ensure fair dispatch")
	}

	msgs, err := r.channel.Consume(
		string(queueName), // queue
		"",                // consumer
		false,             // auto-ack
		false,             // exclusive
		false,             // no-local
		false,             // no-wait
		nil,               // args
	)
	if err != nil {
		return err
	}

	go func() {
		for msg := range msgs {
			// Extract trace context from message headers
			carrier := tracing.AmqpHeadersCarrier(msg.Headers)
			ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)

			ctx, span := r.tracer.Start(ctx, "rabbitmq.consume",
				trace.WithAttributes(
					attribute.String("message.destination", msg.Exchange),
					attribute.String("message.routing_key", msg.RoutingKey),
				),
			)
			defer span.End()

			cfg := retry.DefaultConfig()
			retryErr := retry.WithBackoff(ctx, cfg, func() error {
				return handler(ctx, msg)
			})

			if retryErr != nil {
				tracing.HandleError(span, retryErr)
				log.Warn().Err(retryErr).Str("routing_key", string(msg.RoutingKey)).Msgf("Failed to consume message from %s queue", queueName)

				// Attach failure details to trace context
				headers := amqp.Table{}
				if msg.Headers != nil {
					headers = msg.Headers
				}

				headers["x-death-reason"] = retryErr.Error()
				headers["x-origin-exchange"] = msg.Exchange
				headers["x-origin-routing-key"] = msg.RoutingKey
				headers["x-retry-count"] = cfg.MaxRetries
				msg.Headers = headers

				// Reject message without requeue and send to the DLQ
				err = msg.Reject(false)
				if err != nil {
					tracing.HandleError(span, err)
					log.Warn().Err(err).Str("routing_key", string(msg.RoutingKey)).Msg("Failed to send message to DLQ")
				}

				continue
			}

			// Acknowledge message only when handler succeeds
			if ackErr := msg.Ack(false); ackErr != nil {
				tracing.HandleError(span, ackErr)
				log.Warn().Err(ackErr).Str("routing_key", string(msg.RoutingKey)).Msg("Failed to Ack message")
			}
		}
	}()

	return nil
}

func (r *RabbitMQ) PublishMessage(ctx context.Context, exchange AmqpExchange, routingKey AmqpEvent, msg AmqpMessage) error {
	traceCtx, span := r.tracer.Start(ctx, "rabbitmq.publish",
		trace.WithAttributes(
			attribute.String("message.destination", string(exchange)),
			attribute.String("message.routing_key", string(routingKey)),
		),
	)
	defer span.End()

	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		tracing.HandleError(span, err)
		return fmt.Errorf("Failed to marshal AMQP payload: %v", err)
	}
	payload := amqp.Publishing{
		ContentType:  "text/plain",
		Body:         jsonMsg,
		DeliveryMode: amqp.Persistent,
	}

	// Inject trace context into payload headers
	if payload.Headers == nil {
		payload.Headers = make(amqp.Table)
	}
	carrier := tracing.AmqpHeadersCarrier(payload.Headers)
	otel.GetTextMapPropagator().Inject(traceCtx, carrier)
	payload.Headers = amqp.Table(carrier)

	return r.channel.PublishWithContext(traceCtx,
		string(exchange),   // exchange
		string(routingKey), // routing key
		false,              // mandatory
		false,              // immediate
		payload,
	)
}

func (r *RabbitMQ) setupExchangesAndQueues() error {
	exchanges := []AmqpExchange{GatewayExchange, ServicesExchange, DeadLetterExchange}

	for _, exchange := range exchanges {
		err := r.channel.ExchangeDeclare(
			string(exchange), // name
			"topic",          // type
			true,             // durable
			false,            // auto-deleted
			false,            // internal
			false,            // no-wait
			nil,              // arguments
		)
		if err != nil {
			return fmt.Errorf("Failed to declare exchange: %s: %v", exchange, err)
		}
	}

	if err := r.declareAndBindQueue(
		GatewayQueue,
		[]AmqpEvent{"user.*"},
		GatewayExchange,
	); err != nil {
		return err
	}

	if err := r.declareAndBindQueue(
		DeadLetterQueue,
		[]AmqpEvent{"#"}, // Wildcard to catch all messages
		DeadLetterExchange,
	); err != nil {
		return err
	}

	if err := r.declareAndBindQueue(
		AssignDriverQueue,
		[]AmqpEvent{
			TripEventCreated,
			TripEventDriverNotInterested,
			TripEventDriverNotAvailable,
		},
		ServicesExchange,
	); err != nil {
		return err
	}

	if err := r.declareAndBindQueue(
		DriverUpdateQueue,
		[]AmqpEvent{DriverCmdDetailsUpdate},
		ServicesExchange,
	); err != nil {
		return err
	}

	if err := r.declareAndBindQueue(
		TripUpdateQueue,
		[]AmqpEvent{
			TripEventDriverAssigned,
			TripEventNoDriversFound,
			DriverCmdTripPickup,
			DriverCmdEndTrip,
			TripCmdCancelled,
			TripCmdCompleted,
			TripCmdAborted,
		},
		ServicesExchange,
	); err != nil {
		return err
	}

	if err := r.declareAndBindQueue(
		PaymentQueue,
		[]AmqpEvent{
			PaymentEventWebhookReceived,
			PaymentEventCashReceived,
		},
		ServicesExchange,
	); err != nil {
		return err
	}

	if err := r.declareAndBindQueue(
		AnalyticsQueue,
		[]AmqpEvent{AnalyticsEventUpdate},
		AnalyticsExchange,
	); err != nil {
		return err
	}

	return nil
}

func (r *RabbitMQ) declareAndBindQueue(queueName AmqpQueue, messageTypes []AmqpEvent, exchange AmqpExchange) error {
	// Add dead letter configuration
	args := amqp.Table{
		"x-dead-letter-exchange": DeadLetterExchange,
	}

	q, err := r.channel.QueueDeclare(
		string(queueName), // name
		true,              // durable
		false,             // delete when unused
		false,             // exclusive
		false,             // no-wait
		args,              // arguments with DLX config
	)
	if err != nil {
		return fmt.Errorf("Failed to declare queue: %v", err)
	}

	for _, msg := range messageTypes {
		if err := r.channel.QueueBind(
			q.Name,           // queue name
			string(msg),      // routing key
			string(exchange), // exchange
			false,
			nil,
		); err != nil {
			return fmt.Errorf("Failed to bind %s exchange to %s queue: %v", exchange, q.Name, err)
		}
	}

	return nil
}

func (r *RabbitMQ) Close() {
	if r.conn != nil {
		r.conn.Close()
	}

	if r.channel != nil {
		r.channel.Close()
	}
}
