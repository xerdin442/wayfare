package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/contracts"
)

type RabbitMQ struct {
	conn    *amqp.Connection
	Channel *amqp.Channel
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
		Channel: ch,
	}

	if err := rmq.setupExchangesAndQueues(); err != nil {
		rmq.Close()
		log.Fatal().Err(err).Msg("Failed to setup exchanges and queues")
	}

	return rmq
}

func (r *RabbitMQ) ConsumeMessages(name QueueName, handler func(context.Context, amqp.Delivery) error) error {
	// Ensure fair dispatch
	err := r.Channel.Qos(
		1,     // prefetchCount
		0,     // prefetchSize
		false, // global
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to set QoS and ensure fair dispatch")
	}

	msgs, err := r.Channel.Consume(
		string(name), // queue
		"",           // consumer
		false,        // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		return err
	}

	go func() {
		for msg := range msgs {
			if err := handler(context.Background(), msg); err != nil {
				log.Warn().Err(err).Str("message", string(msg.Body)).Msgf("Failed to consume message from %s queue", name)
				if nackErr := msg.Nack(false, false); nackErr != nil {
					log.Warn().Err(nackErr).Str("message", string(msg.Body)).Msg("Failed to Nack message")
				}

				continue
			}

			// Acknowledge message only when handler succeeds
			if ackErr := msg.Ack(false); ackErr != nil {
				log.Warn().Err(ackErr).Str("message", string(msg.Body)).Msg("Failed to Ack message")
			}
		}
	}()

	return nil
}

func (r *RabbitMQ) PublishMessage(ctx context.Context, routingKey contracts.AmqpEvent, message contracts.AmqpMessage) error {
	jsonMsg, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("Failed to marshal AMQP message: %v", err)
	}

	return r.Channel.PublishWithContext(ctx,
		string(TripExchange), // exchange
		string(routingKey),   // routing key
		false,                // mandatory
		false,                // immediate
		amqp.Publishing{
			ContentType:  "text/plain",
			Body:         jsonMsg,
			DeliveryMode: amqp.Persistent,
		})
}

func (r *RabbitMQ) setupExchangesAndQueues() error {
	err := r.Channel.ExchangeDeclare(
		string(TripExchange), // name
		"topic",              // type
		true,                 // durable
		false,                // auto-deleted
		false,                // internal
		false,                // no-wait
		nil,                  // arguments
	)
	if err != nil {
		return fmt.Errorf("Failed to declare exchange: %s: %v", TripExchange, err)
	}

	if err := r.declareAndBindQueue(
		FindAvailableDriversQueue,
		[]contracts.AmqpEvent{
			contracts.TripEventCreated, contracts.TripEventDriverNotInterested,
		},
		TripExchange,
	); err != nil {
		return err
	}

	return nil
}

func (r *RabbitMQ) declareAndBindQueue(name QueueName, messageTypes []contracts.AmqpEvent, exchange AmqpExchange) error {
	q, err := r.Channel.QueueDeclare(
		string(name), // name
		true,         // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		return fmt.Errorf("Failed to declare queue")
	}

	for _, msg := range messageTypes {
		if err := r.Channel.QueueBind(
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

	if r.Channel != nil {
		r.Channel.Close()
	}
}
