package secrets

import (
	"os"
	"strconv"

	_ "github.com/joho/godotenv/autoload"
	"github.com/rs/zerolog/log"
)

type Secrets struct {
	GatewayPort int
	ServicePort int
	Environment string
	RedisUri    string
	MongoUri    string
	AmqpUri     string
	JwtSecret   string
	FrontendUrl string
}

func Load() *Secrets {
	return &Secrets{
		GatewayPort: GetInt("GATEWAY_PORT"),
		ServicePort: GetInt("SERVICE_PORT"),
		Environment: GetStr("ENVIRONMENT"),
		RedisUri:    GetStr("REDIS_URI"),
		MongoUri:    GetStr("MONGO_URI"),
		AmqpUri:     GetStr("AMQP_URI"),
		JwtSecret:   GetStr("JWT_SECRET"),
		FrontendUrl: GetStr("FRONTEND_URL"),
	}
}

func GetStr(key string) string {
	value, ok := os.LookupEnv(key)

	if !ok {
		log.Fatal().Msgf("Missing environment variable: %s", key)
	}

	return value
}

func GetInt(key string) int {
	strValue := GetStr(key)

	intValue, err := strconv.Atoi(strValue)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid string value")
	}

	return int(intValue)
}
