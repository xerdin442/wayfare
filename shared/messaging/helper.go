package messaging

type AmqpExchange string

const (
	TripExchange       AmqpExchange = "trips"
	DeadLetterExchange AmqpExchange = "dlx"
)

type QueueName string

const (
	FindAvailableDriversQueue QueueName = "find_available_drivers"
)
