package tracing

import amqp "github.com/rabbitmq/amqp091-go"

// AmqpHeadersCarrier implements the TextMapCarrier interface for AMQP headers
type AmqpHeadersCarrier amqp.Table

func (c AmqpHeadersCarrier) Get(key string) string {
	if v, ok := c[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (c AmqpHeadersCarrier) Set(key string, value string) {
	c[key] = value
}

func (c AmqpHeadersCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}
