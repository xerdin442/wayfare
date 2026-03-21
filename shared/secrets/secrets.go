package secrets

import (
	"os"
	"strconv"

	_ "github.com/joho/godotenv/autoload"
	"github.com/rs/zerolog/log"
)

type Secrets struct {
	Port            int
	Environment     string
	RedisUri        string
	MongoUri        string
	JwtSecret       string
	FrontendUrl     string
	TripServiceAddr string
}

func Load() *Secrets {
	return &Secrets{
		Port:            GetInt("PORT"),
		Environment:     GetStr("ENVIRONMENT"),
		RedisUri:        GetStr("REDIS_URI"),
		MongoUri:        GetStr("MONGO_URI"),
		JwtSecret:       GetStr("JWT_SECRET"),
		FrontendUrl:     GetStr("FRONTEND_URL"),
		TripServiceAddr: GetStr("TRIP_SERVICE_ADDR"),
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
