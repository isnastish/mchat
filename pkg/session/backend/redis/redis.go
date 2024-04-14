package redis

import (
	"context"
	"github.com/redis/go-redis/v9"
	"time"

	lgr "github.com/isnastish/chat/pkg/logger"
)

type RedisSettings struct {
	Network    string
	Addr       string
	Password   string
	MaxRetries int
}

type RedisBackend struct {
	client *redis.Client
	ctx    context.Context
}

var log = lgr.NewLogger("debug")

func NewRedisBackend(settings *RedisSettings) (*RedisBackend, error) {
	options := redis.Options{
		Network:    settings.Network,
		Addr:       settings.Addr,
		Password:   settings.Password,
		MaxRetries: settings.MaxRetries,
		DB:         0,
		OnConnect: func(ctx context.Context, cn *redis.Conn) error {
			log.Info().Msg("Connection with redis server established")
			return nil
		},
	}

	client := redis.NewClient(&options)
	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	rb := &RedisBackend{
		client: client,
		ctx:    ctx,
	}

	return rb, nil
}

func (b *RedisBackend) HasClient(name string) bool {
	return true
}

func (b *RedisBackend) MatchClientPassword(name string, password string) bool {
	return true
}

func (b *RedisBackend) RegisterNewClient(name string, addr string, status string, password string, connTime time.Time) bool {
	return true
}

func (b *RedisBackend) AddMessage(name string, contents_ []byte, sentTime_ time.Time) bool {
	return true
}
