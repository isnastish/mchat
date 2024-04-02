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
	Network    string
	Addr       string
	Password   string
	MaxRetries int
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

func (rb *RedisBackend) HasClient(name string) bool {
	val := rb.client.HGetAll(rb.ctx, name).Val()
	return len(val) != 0
}

func (rb *RedisBackend) RegisterClient(name string, ipAddress string, status string, joinedTime time.Time) error {
	var m = map[string]string{
		"ip_address": ipAddress,
		"status":     status,
		"joinedTime": joinedTime.Format(time.DateTime),
	}

	if err := rb.client.HSet(rb.ctx, name, m).Err(); err != nil {
		return err
	}

	return nil
}

func (rb *RedisBackend) AddMessage(clientName string, sentTime time.Time, body [1024]byte) {

}

func (rb *RedisBackend) GetClients() (map[string]*map[string]string, error) {
	return nil, nil
}
