package base

import (
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/client"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/secrets"
)

type Config struct {
	Env         *secrets.Secrets
	Cache       *redis.Client
	ConnManager sync.Map
	Clients     *client.Registry
	Queue       messaging.MessageBus
}
