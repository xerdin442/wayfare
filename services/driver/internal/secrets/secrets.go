package secrets

import (
	"os"
	"strconv"

	_ "github.com/joho/godotenv/autoload"
	"github.com/rs/zerolog/log"
)

type Secrets struct {
	ServicePort            int
	Environment            string
	RedisUri               string
	MongoUri               string
	AmqpUri                string
	TraceCollectorEndpoint string
}

func Load() *Secrets {
	return &Secrets{
		ServicePort:            getInt("SERVICE_PORT"),
		Environment:            getStr("ENVIRONMENT"),
		RedisUri:               getStr("REDIS_URI"),
		MongoUri:               getStr("MONGO_URI"),
		AmqpUri:                getStr("AMQP_URI"),
		TraceCollectorEndpoint: getStr("TRACE_COLLECTOR_ENDPOINT"),
	}
}

func getStr(key string) string {
	value, ok := os.LookupEnv(key)

	if !ok {
		log.Fatal().Msgf("Missing environment variable: %s", key)
	}

	return value
}

func getInt(key string) int {
	strValue := getStr(key)

	intValue, err := strconv.Atoi(strValue)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid string value")
	}

	return int(intValue)
}
