package redis

import (
	"context"
	"sync"
	"time"

	backend "github.com/isnastish/chat/pkg/session/backend"
	"github.com/redis/go-redis/v9"
)

type RedisBackend struct {
	client *redis.Client
	// every redis command uses context to set a timeout or to propagate tracing information
	// using prometheus or grafana
	ctx context.Context
	mu  sync.Mutex
}

// type participant struct {
// 	username       string `redis:"username"`
// 	passwordSha256 string `redis:"passwordSha256"`
// 	emailAddress   string `redis:"emailAddress"`
// 	joinTime       string `redis:"joinTime"`
// }

func NewBackend() *RedisBackend {
	client := redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "",
		DB:       0,
	})

	return &RedisBackend{
		client: client,
		ctx:    context.Background(),
	}
}

func (redis *RedisBackend) HasParticipant(username string) bool {
	return false
}

func (redis *RedisBackend) RegisterParticipant(username string, passwordShaw256 string, emailAddress string) {
	redis.mu.Lock()

	// STRINGs, LISTs, SETs, HASHes, and ZSETs.

	redis.client.HSet(
		redis.ctx,
		username,
		[]string{username, passwordShaw256, emailAddress, time.Now().Format(time.DateTime)},
	)

	data := redis.client.HGetAll(redis.ctx, username)
	_ = data

	redis.mu.Unlock()
}

func (b *RedisBackend) AuthParticipant(username string, passwordSha256 string) bool {
	return false
}

func (b *RedisBackend) StoreMessage(senderName string, sentTime string, contents []byte, channelName ...string) {

}

func (b *RedisBackend) HasChannel(channelName string) bool {
	return false
}

func (b *RedisBackend) RegisterChannel(channelName string, desc string, ownerName string) {
}

func (b *RedisBackend) DeleteChannel(channelName string) bool {
	return false
}

func (b *RedisBackend) GetChatHistory(channelName ...string) []*backend.ParticipantMessage {
	return nil
}

func (b *RedisBackend) GetChannelHistory(channelName string) {

}

func (b *RedisBackend) GetChannels() []*backend.Channel {
	return nil
}

func (b *RedisBackend) GetParticipantList() []*backend.Participant {
	return nil
}
