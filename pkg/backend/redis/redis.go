package redis

import (
	"context"
	"sync"

	"github.com/redis/go-redis/v9"

	"github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/types"
	"github.com/isnastish/chat/pkg/utilities"
	"github.com/isnastish/chat/pkg/validation"
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

// HasParticipant(username string) bool
// RegisterParticipant(participant *types.Participant)
// AuthParticipant(participant *types.Participant) bool
// StoreMessage(message *types.ChatMessage)
// HasChannel(channelname string) bool
// RegisterChannel(channel *types.Channel)
// DeleteChannel(channelname string) bool
// GetChatHistory()
//
// GetChannels() []*types.Channel
// GetParticipantList() []*types.Participant

func (r *RedisBackend) HasParticipant(username string) bool {
	return false
}

func (r *RedisBackend) RegisterParticipant(participant *types.Participant) {
	r.mu.Lock()

	// TODO: Check whether a participant already exists first
	passwordHash := utilities.Sha256Checksum([]byte(participant.Password))
	if validation.ValidatePasswordSha256(passwordHash) {
		log.Logger.Panic("Failed to register participant %s. Password validation failed", participant.Username)
	}

	// r.client.HSet(
	// 	r.ctx,
	// 	username,
	// 	[]string{username, passwordShaw256, emailAddress, time.Now().Format(time.DateTime)},
	// )

	// data := r.client.HGetAll(r.ctx, username)
	// _ = data

	r.mu.Unlock()
}

func (r *RedisBackend) AuthParticipant(participant *types.Participant) bool {
	return false
}

func (r *RedisBackend) StoreMessage(message *types.ChatMessage) {

}

func (r *RedisBackend) HasChannel(channelname string) bool {
	return false
}

func (r *RedisBackend) RegisterChannel(channel *types.Channel) {
}

func (r *RedisBackend) DeleteChannel(channelname string) bool {
	return false
}

func (r *RedisBackend) GetChatHistory() []*types.ChatMessage {
	return nil
}

func (r *RedisBackend) GetChannelHistory(channelname string) []*types.ChatMessage {
	return nil
}

func (r *RedisBackend) GetChannels() []*types.Channel {
	return nil
}

func (r *RedisBackend) GetParticipantList() []*types.Participant {
	return nil
}
