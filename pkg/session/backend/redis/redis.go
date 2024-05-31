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
	ctx    context.Context
	mu     sync.Mutex
}

func NewRedisBackend(addr string) (*RedisBackend, error) {
	options := &redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	}

	client := redis.NewClient(options)

	rb := &RedisBackend{
		client: client,
		ctx:    context.Background(),
	}

	return rb, nil
}

func (redis *RedisBackend) HasParticipant(username string) bool {
	return false
}

func (r *RedisBackend) RegisterParticipant(username string, passwordShaw256 string, emailAddress string) {
	r.mu.Lock()

	r.client.HSet(
		r.ctx,
		username,
		[]string{username, passwordShaw256, emailAddress, time.Now().Format(time.DateTime)},
	)

	data := r.client.HGetAll(r.ctx, username)
	_ = data

	r.mu.Unlock()
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
