// TODO: Figure out how to persist the data if the redis server gets of.
// Maybe the data should be replicated on disk after each operation: RegisterParticipant/Channel etc.
// TODO: Implement metrics using prometheus or grafana.
// TODO: Explore Redis' transactions, maybe we wouldn't have to maintain a mutex.

package redis

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"time"

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

func (r *RedisBackend) doesParticipantExist(participantUsername string) bool {
	isMember := r.client.SIsMember(r.ctx, "participants:", participantUsername)
	return isMember.Val()
}

func (r *RedisBackend) doesChannelExist(channelname string) bool {
	isMember := r.client.SIsMember(r.ctx, "channels:", channelname)
	return isMember.Val()
}

func (r *RedisBackend) HasParticipant(username string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.doesParticipantExist(username)
}

func (r *RedisBackend) RegisterParticipant(participant *types.Participant) {
	r.mu.Lock()
	defer r.mu.Unlock()

	passwordHash := utilities.Sha256Checksum([]byte(participant.Password))
	if !validation.ValidatePasswordSha256(passwordHash) {
		log.Logger.Panic("Failed to register participant %s. Password validation failed", participant.Username)
	}

	if r.doesParticipantExist(participant.Username) {
		log.Logger.Panic("Participant %s already exists", participant.Username)
	}

	// Not sure whether we need to hash a participant's username in order to use it as a key.
	participantHash := utilities.Sha256Checksum([]byte(participant.Username))

	value := reflect.ValueOf(participant).Elem()
	for i := 0; i < value.NumField(); i++ {
		fieldname := value.Type().Field(i).Name
		fieldvalue := value.Field(i).Interface()
		// NOTE: This has to be in sync with types.Participant Password field.
		// If that changes, the code breaks. Cannot think of more general solution
		// of how to solve this problem (inserting only hashed paswords).
		// This relies on the order of fields inside types.Participant, not on their names itself.
		// Maybe the best way to solve it would be to use tags.
		if i == 1 {
			fieldvalue = passwordHash
		}

		r.client.HSet(r.ctx, participantHash, fieldname, fieldvalue)
	}

	if len(r.client.HGetAll(r.ctx, participantHash).Val()) != 0 {
		r.client.SAdd(r.ctx, "participants:", participant.Username)
		log.Logger.Info("Registered %s participant", participant.Username)
	} else {
		log.Logger.Info("Failed to register participant %s", participant.Username)
	}
}

// NOTE: Not a part of a public API yet.
func (r *RedisBackend) deleteParticipant(participant *types.Participant) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.doesParticipantExist(participant.Username) {
		r.client.SRem(r.ctx, "participants:", participant.Username)
		participantHash := utilities.Sha256Checksum([]byte(participant.Username))
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
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.doesChannelExist(channelname)
}

func (r *RedisBackend) RegisterChannel(channel *types.Channel) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.doesChannelExist(channel.Name) {
		log.Logger.Panic("Channel %s already exists", channel.Name)
	}

	channelHash := utilities.Sha256Checksum([]byte(channel.Name))

	value := reflect.ValueOf(channel).Elem()
	for i := 0; i < value.NumField(); i++ {
		// Skip chat history and members for now
		if value.Field(i).Type() == reflect.TypeOf(channel.ChatHistory) ||
			value.Field(i).Type() == reflect.TypeOf(channel.Members) {
			continue
		}

		fieldname := value.Type().Field(i).Name
		fieldvalue := value.Field(i).Interface()

		r.client.HSet(r.ctx, channelHash, fieldname, fieldvalue)
	}

	if len(r.client.HGetAll(r.ctx, channelHash).Val()) != 0 {
		r.client.SAdd(r.ctx, "channels:", channel.Name)
		log.Logger.Info("Registered %s channel", channel.Name)
	} else {
		log.Logger.Info("Failed to register channel %s", channel.Name)
	}
}

func (r *RedisBackend) DeleteChannel(channelname string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.doesChannelExist(channelname) {
		r.client.SRem(r.ctx, "channels:", channelname)
		channelHash := utilities.Sha256Checksum([]byte(channelname))
		r.client.Del(r.ctx, channelHash)
		log.Logger.Info("Channel %s was deleted", channelname)

		return true
	}

	return false
}

func (r *RedisBackend) GetChatHistory() []*types.ChatMessage {

	return nil
}

func (r *RedisBackend) GetChannelHistory(channelname string) []*types.ChatMessage {
	return nil
}

func (r *RedisBackend) GetChannels() []*types.Channel {
	r.mu.Lock()
	defer r.mu.Unlock()

	members := r.client.SMembers(r.ctx, "channels:")
	if len(members.Val()) == 0 {
		return nil
	}

	channels := make([]*types.Channel, 0, len(members.Val()))

	for _, channelName := range members.Val() {
		channelHash := utilities.Sha256Checksum([]byte(channelName))

		data := r.client.HGetAll(r.ctx, channelHash)
		if len(data.Val()) == 0 {
			log.Logger.Panic("Channel %s was't found", channelName)
		}

		channel := &types.Channel{}

		value := reflect.ValueOf(channel).Elem()
		for i := 0; i < value.NumField(); i++ {
			fieldname := value.Type().Field(i).Name
			if value.Field(i).CanSet() {
				fieldtype := value.Field(i).Type()
				if fieldtype == reflect.TypeOf(channel.Members) ||
					fieldtype == reflect.TypeOf(channel.ChatHistory) ||
					fieldtype == reflect.TypeOf(channel.CreationDate) {
					continue
				}
				value.Field(i).Set(reflect.ValueOf(data.Val()[fieldname]))
			}
		}
		channel = (*types.Channel)(value.Addr().UnsafePointer())
		channels = append(channels, channel)
	}
	return channels
}

func (r *RedisBackend) GetParticipants() []*types.Participant {
	r.mu.Lock()
	defer r.mu.Unlock()

	// TODO: Figure out how to use redis transactions in order to speed up the performance
	// (reduce the amount of calls to the redis server)
	members := r.client.SMembers(r.ctx, "participants:")
	if len(members.Val()) == 0 {
		return nil
	}

	participants := make([]*types.Participant, 0, len(members.Val()))

	for _, participantUsername := range members.Val() {
		participantHash := utilities.Sha256Checksum([]byte(participantUsername))

		data := r.client.HGetAll(r.ctx, participantHash)
		if len(data.Val()) == 0 {
			log.Logger.Panic("Participant %s not found", participantUsername)
		}

		participant := &types.Participant{}

		value := reflect.ValueOf(participant).Elem()
		for i := 0; i < value.NumField(); i++ {
			fieldname := value.Type().Field(i).Name
			if value.Field(i).CanSet() {
				if value.Field(i).Type() == reflect.TypeOf(participant.JoinTime) {
					value.Field(i).Set(reflect.ValueOf(time.Now()))
					continue
				}
				value.Field(i).Set(reflect.ValueOf(data.Val()[fieldname]))
			}
		}
		participant = (*types.Participant)(value.Addr().UnsafePointer())
		participants = append(participants, participant)
	}
	return participants
}
