package redis

import (
	"context"
	"sync"

	backend "github.com/isnastish/chat/pkg/session/backend"
	"github.com/redis/go-redis/v9"
)

type RedisBackend struct {
	client *redis.Client
	ctx    context.Context
	mu     sync.Mutex
}

func NewBackend() *RedisBackend {

	return &RedisBackend{}
}

func (b *RedisBackend) HasParticipant(username string) bool {
	return false
}

func (b *RedisBackend) RegisterParticipant(username string, passwordShaw256 string, emailAddress string) {

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
