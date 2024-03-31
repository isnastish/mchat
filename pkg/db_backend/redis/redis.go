package redis

import (
	"context"
	"github.com/redis/go-redis/v9"
	"time"

	lgr "github.com/isnastish/chat/pkg/logger"
)

type RedisBackend struct {
	client *redis.Client
	ctx    context.Context
}

type RedisSettings struct {
	network    string
	address    string
	password   string
	maxRetries int
}

var log = lgr.NewLogger("debug")

func NewRedisBackend(settings *RedisSettings) (*RedisBackend, error) {
	options := redis.Options{
		Network:    settings.network,
		Addr:       settings.address,
		Password:   settings.password,
		MaxRetries: settings.maxRetries,
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

// func (r *Redis) storeMap(m map[string]*map[string]string) error {
// 	for hash, subMap := range m {
// 		for k, v := range *subMap {
// 			err := r.client.HSet(r.ctx, hash, k, v).Err()
// 			if err != nil {
// 				return err
// 			}
// 		}
// 	}
// }

// func (r *Redis) getMap(hash string) (map[string]string, error) {
// 	res := r.client.HGetAll(r.ctx, hash)
// 	if err := res.Err(); err != nil {
// 		return nil, err
// 	}
// 	return res.Val(), nil
// }

func (rb *RedisBackend) ContainsClient(identifier string) bool {
	val := rb.client.HGetAll(rb.ctx, identifier).Val()
	return len(val) != 0
}

func (rb *RedisBackend) RegisterClient(identifier string, ipAddress string, status string, joinedTime time.Time) bool {
	return true
}

func (rb *RedisBackend) UpdateClient(identifier string, rest ...any) bool {
	return true
}

func (rb *RedisBackend) GetParticipantsList() ([][3]string, error) {
	return nil, nil
}
