// TODO: Figure out how to persist the data if the redis server gets of.
// Maybe the data should be replicated on disk after each operation: RegisterParticipant/Channel etc.
// TODO: Implement metrics using prometheus or grafana.
// NOTE: redis.Del(r.ctx, key) - deletes the hash itself
package redis

import (
	"context"
	"reflect"
	"strings"
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

	participantHash := utilities.Sha256Checksum([]byte(participant.Username))

	if r.doesParticipantExist(participantHash) {
		log.Logger.Panic("Participant %s already exists", participant.Username)
	}

	value := reflect.ValueOf(participant).Elem()
	for i := 0; i < value.NumField(); i++ {
		fieldname := value.Type().Field(i).Name
		fieldvalue := value.Field(i).Interface()
		// NOTE: This has to be in sync with types.Participant Password field.
		// If that changes, the code breaks. Cannot think of more general solution
		// of how to solve this problem (inserting only hashed paswords).
		// This relies on the order of fields inside types.Participant, not on their names itself.
		if i == 1 {
			fieldvalue = passwordHash
		}

		r.client.HSet(r.ctx, participantHash, fieldname, fieldvalue)
	}

	if r.doesParticipantExist(participantHash) {
		log.Logger.Info("Registered %s participant", participant.Username)
	} else {
		log.Logger.Info("Failed to register participant %s", participant.Username)
	}
}

func (r *RedisBackend) DeleteParticipant(participant *types.Participant) {
	r.mu.Lock()
	defer r.mu.Unlock()

	participantHash := utilities.Sha256Checksum([]byte(participant.Username))
	if r.doesParticipantExist(participantHash) {
		r.client.Del(r.ctx, participantHash)
		log.Logger.Info("Participant %s was deleted", participant.Username)
	}
}

func (r *RedisBackend) AuthParticipant(participant *types.Participant) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	participantHash := utilities.Sha256Checksum([]byte(participant.Username))

	if r.doesParticipantExist(participantHash) {
		// NOTE: This has to be in sync with types.Participant struct because it relies on the order of fields.
		// Field(1) is expected to correspond to the Password field inside that struct.
		passwordFiledName := reflect.TypeOf(participant).Elem().Field(1).Name
		passwordHash := r.client.HGet(r.ctx, participantHash, passwordFiledName)
		if passwordHash.Val() != "" {
			return strings.EqualFold(utilities.Sha256Checksum([]byte(participant.Password)), passwordHash.Val())
		}
	}

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
