// TODO: Figure out how to persist the data if the redis server gets of.
// Maybe the data should be replicated on disk after each operation: RegisterParticipant/Channel etc.
// TODO: Implement metrics using prometheus or grafana.
// NOTE: redis.Del(r.ctx, key) - deletes the hash itself
package redis

import (
	"context"
	"reflect"
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

func (r *RedisBackend) doesParticipantExist(participantHash string) bool {
	result := r.client.HGetAll(r.ctx, participantHash)
	return len(result.Val()) != 0
}

// func (r *RedisBackend) deleteParticipant(participantHash string, participant *types.Participant) {
// 	value := reflect.ValueOf(participant).Elem()
// 	for i := 0; i < value.NumField(); i++ {
// 		fieldname := value.Type().Field(i).Name
// 		r.client.HDel(r.ctx, participantHash, fieldname)
// 	}
// }

func (r *RedisBackend) HasParticipant(username string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.doesParticipantExist(utilities.Sha256Checksum([]byte(username)))
}

func (r *RedisBackend) RegisterParticipant(participant *types.Participant) {
	r.mu.Lock()
	defer r.mu.Unlock()

	passwordHash := utilities.Sha256Checksum([]byte(participant.Password))
	if !validation.ValidatePasswordSha256(passwordHash) {
		log.Logger.Panic("Failed to register participant %s. Password validation failed", participant.Username)
	}

	// Should we validate participant's hash?
	participantHash := utilities.Sha256Checksum([]byte(participant.Username))

	value := reflect.ValueOf(participant).Elem()
	for i := 0; i < value.NumField(); i++ {
		fieldname := value.Type().Field(i).Name
		fieldvalue := value.Field(i).Interface()

		r.client.HSet(r.ctx, participantHash, fieldname, fieldvalue)
	}

	if r.doesParticipantExist(participantHash) {
		log.Logger.Info("Registered %s participant", participant.Username)
	} else {
		log.Logger.Info("Failed to register participant %s", participant.Username)
	}
}

func (r *RedisBackend) AuthParticipant(participant *types.Participant) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// participantHash := utilities.Sha256Checksum([]byte(participant.Username))

	// passwordHash := utilities.Sha256Checksum([]byte(participant.Password))
	// return strings.EqualFold(participant.Password, passwordHash)

	// TODO: Verify if Val() is an empty string.
	// if r.doesParticipantExist(participantHash) {
	// 	passwordFiledName := reflect.ValueOf(participant.Password).Type().Name()
	// 	password := r.client.HGet(r.ctx, participantHash, passwordFiledName)
	// 	if password.Val() != "" {

	// 	}
	// }

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
