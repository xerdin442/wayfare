package secrets

import (
	"os"
	"strconv"

	_ "github.com/joho/godotenv/autoload"
	"github.com/rs/zerolog/log"
)

type Secrets struct {
	GatewayPort            int
	Environment            string
	RedisUri               string
	AmqpUri                string
	TraceCollectorEndpoint string
	JwtSecret              string
	FrontendUrl            string
	CloudinaryName         string
	CloudinarySecret       string
	CloudinaryApiKey       string
	PaystackApiUrl         string
	PaystackSecretKey      string
	FlutterwaveVerifHash   string
	ClickHouseUri          string
	ClickHouseUsername     string
	ClickHousePassword     string
}

func Load() *Secrets {
	return &Secrets{
		GatewayPort:            getInt("GATEWAY_PORT"),
		Environment:            getStr("ENVIRONMENT"),
		RedisUri:               getStr("REDIS_URI"),
		AmqpUri:                getStr("AMQP_URI"),
		TraceCollectorEndpoint: getStr("TRACE_COLLECTOR_ENDPOINT"),
		JwtSecret:              getStr("JWT_SECRET"),
		FrontendUrl:            getStr("FRONTEND_URL"),
		CloudinaryName:         getStr("CLOUDINARY_NAME"),
		CloudinarySecret:       getStr("CLOUDINARY_SECRET"),
		CloudinaryApiKey:       getStr("CLOUDINARY_API_KEY"),
		PaystackApiUrl:         getStr("PAYSTACK_API_URL"),
		PaystackSecretKey:      getStr("PAYSTACK_SECRET_KEY"),
		FlutterwaveVerifHash:   getStr("FLUTTERWAVE_VERIF_HASH"),
		ClickHouseUri:          getStr("CLICKHOUSE_URI"),
		ClickHouseUsername:     getStr("ROOT_USERNAME"),
		ClickHousePassword:     getStr("ROOT_PASSWORD"),
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
