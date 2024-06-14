// TODO: Figure out how to persist the data if the redis server gets of.
// Maybe the data should be replicated on disk after each operation: RegisterParticipant/Channel etc.
// TODO: Implement metrics using prometheus or grafana.
// TODO: Explore Redis' transactions, maybe we wouldn't have to maintain a mutex.
// TODO: GetChatHistory and GetChannelHistory have some duplications, they have to be combined into a single function.
// TODO: Add metrics using otel (opentelementry).
// TODO: Don't hash username and channel name when inserting into redis
package redis

import (
	"bytes"
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/isnastish/chat/pkg/backend"
	"github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/types"
	"github.com/isnastish/chat/pkg/utilities"
	"github.com/isnastish/chat/pkg/validation"
)

type Config struct {
	Addr string
	Port int
}

type redisBackend struct {
	client *redis.Client
	ctx    context.Context
	sync.RWMutex
}

func NewRedisBackend(config *backend.RedisConfig) (*redisBackend, error) {
	options := &redis.Options{
		Addr:     config.Endpoint,
		Password: "",
	}

	client := redis.NewClient(options)

	rb := &redisBackend{
		client: client,
		ctx:    context.Background(),
	}

	statusCode := client.Ping(rb.ctx)

	return rb, statusCode.Err()
}

func (r *redisBackend) doesParticipantExist(participantUsername string) bool {
	isMember := r.client.SIsMember(r.ctx, "participants:", participantUsername)
	return isMember.Val()
}

func (r *redisBackend) doesChannelExist(channelname string) bool {
	isMember := r.client.SIsMember(r.ctx, "channels:", channelname)
	return isMember.Val()
}

func (r *redisBackend) HasParticipant(username string) bool {
	// NOTE: Read lock will block until any open write lock is released.
	// The Lock() method of the write lock will block if another process has either read or write lock
	// until that lock is released.
	r.RLock()
	defer r.RUnlock()
	return r.doesParticipantExist(username)
}

func (r *redisBackend) RegisterParticipant(participant *types.Participant) {
	r.Lock()
	defer r.Unlock()

	passwordHash := util.Sha256Checksum([]byte(participant.Password))
	if !validation.ValidatePasswordSha256(passwordHash) {
		log.Logger.Panic("Failed to register participant %s. Password validation failed", participant.Username)
	}

	// r.client.Del(r.ctx, "participants:")

	if r.doesParticipantExist(participant.Username) {
		log.Logger.Panic("Participant %s already exists", participant.Username)
	}

	// Not sure whether we need to hash a participant's username in order to use it as a key.
	participantHash := util.Sha256Checksum([]byte(participant.Username))

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
func (r *redisBackend) deleteParticipant(username string) {
	r.Lock()
	defer r.Unlock()

	if r.doesParticipantExist(username) {
		r.client.SRem(r.ctx, "participants:", username)
		participantHash := util.Sha256Checksum([]byte(username))
		r.client.Del(r.ctx, participantHash)

		log.Logger.Info("Participant %s was deleted", username)
	}
}

func (r *redisBackend) AuthParticipant(participant *types.Participant) bool {
	r.RLock()
	defer r.RUnlock()

	if r.doesParticipantExist(participant.Username) {
		participantHash := util.Sha256Checksum([]byte(participant.Username))

		// NOTE: This has to be in sync with types.Participant struct because it relies on the order of fields.
		// Field(1) is expected to correspond to the Password field inside that struct.
		passwordFiledName := reflect.TypeOf(participant).Elem().Field(1).Name
		passwordHash := r.client.HGet(r.ctx, participantHash, passwordFiledName)
		if passwordHash.Val() != "" {
			return util.Sha256Checksum([]byte(participant.Password)) == passwordHash.Val()
		}
	}

	return false
}

func (r *redisBackend) StoreMessage(message *types.ChatMessage) {
	r.Lock()
	defer r.Unlock()

	var messagesKey string
	var messageId string

	if message.Channel != "" {
		if !r.doesChannelExist(message.Channel) {
			log.Logger.Panic("Failed to store a message, channel %s doesn't exist", message.Channel)
		}
		messagesKey = "messages/" + message.Channel + ":"
		messageId = message.Sender + ":" + time.Now().Format(time.DateTime)
		r.client.SAdd(r.ctx, messagesKey, messageId)
	} else {
		messagesKey = "messages/general:"
		messageId = message.Sender + ":" + time.Now().Format(time.DateTime)
		r.client.SAdd(r.ctx, messagesKey, messageId)
	}

	value := reflect.ValueOf(message).Elem()
	for i := 0; i < value.NumField(); i++ {
		fieldname := value.Type().Field(i).Name
		fieldvalue := value.Field(i).Interface()
		if value.Field(i).Type() == reflect.TypeOf(message.Contents) {
			fieldvalue = message.Contents.String()
		}
		r.client.HSet(r.ctx, messageId, fieldname, fieldvalue)
	}

	if len(r.client.HGetAll(r.ctx, messageId).Val()) != 0 {
		log.Logger.Info("Message was stored")
	} else {
		log.Logger.Info("Failed to store the message")
	}
}

// NOTE: Not a part of a public API yet.
func (r *redisBackend) deleteMessages(channels ...string) {
	r.Lock()
	defer r.Unlock()

	if len(channels) != 0 {
		for _, chName := range channels {
			if !r.doesChannelExist(chName) {
				log.Logger.Panic("Cannot delete messages, channel %s doesn't exist", chName)
			}

			channelMessagesKey := "messages/" + chName + ":"
			messages := r.client.SMembers(r.ctx, channelMessagesKey).Val()

			for _, messageId := range messages {
				if len(r.client.HGetAll(r.ctx, messageId).Val()) == 0 {
					log.Logger.Panic("Message id:%s doesn't exist", messageId)
				}
				r.client.Del(r.ctx, messageId)
			}
			r.client.SPopN(r.ctx, channelMessagesKey, int64(len(messages)))
		}
	} else {
		generalMessagesKey := "messages/general:"
		members := r.client.SMembers(r.ctx, generalMessagesKey).Val()
		for _, messageId := range members {
			r.client.SRem(r.ctx, generalMessagesKey, messageId)
			r.client.Del(r.ctx, messageId)
		}
		r.client.SPopN(r.ctx, generalMessagesKey, int64(len(members)))

		log.Logger.Info("All messages were deleted in a general chat")
	}
}

func (r *redisBackend) HasChannel(channelname string) bool {
	r.RLock()
	defer r.RUnlock()
	return r.doesChannelExist(channelname)
}

func (r *redisBackend) RegisterChannel(channel *types.Channel) {
	r.Lock()
	defer r.Unlock()

	if r.doesChannelExist(channel.Name) {
		log.Logger.Panic("Channel %s already exists", channel.Name)
	}

	channelHash := util.Sha256Checksum([]byte(channel.Name))

	value := reflect.ValueOf(channel).Elem()
	for i := 0; i < value.NumField(); i++ {
		// Skip chat history and members for now
		if value.Field(i).Type() == reflect.TypeOf(channel.ChatHistory) ||
			value.Field(i).Type() == reflect.TypeOf(channel.Members) {
			continue
		}

		r.client.HSet(r.ctx, channelHash, value.Type().Field(i).Name, value.Field(i).Interface())
	}

	if len(r.client.HGetAll(r.ctx, channelHash).Val()) != 0 {
		r.client.SAdd(r.ctx, "channels:", channel.Name)
		log.Logger.Info("Registered %s channel", channel.Name)
	} else {
		log.Logger.Info("Failed to register channel %s", channel.Name)
	}
}

func (r *redisBackend) DeleteChannel(channelname string) bool {
	r.Lock()
	defer r.Unlock()

	if r.doesChannelExist(channelname) {
		r.client.SRem(r.ctx, "channels:", channelname)
		channelHash := util.Sha256Checksum([]byte(channelname))
		r.client.Del(r.ctx, channelHash)
		log.Logger.Info("Channel %s was deleted", channelname)

		return true
	}

	return false
}

func (r *redisBackend) GetChatHistory(channelname ...string) []*types.ChatMessage {
	r.RLock()
	defer r.RUnlock()

	var messagesKey string

	// Empty ("") channels name is treated the same as the channel not being specified,
	// thus we have to return general chat's history
	if len(channelname) > 0 && channelname[0] != "" {
		if !r.doesChannelExist(channelname[0]) {
			log.Logger.Panic("Failed to retrieve chat history, channel %s doesn't exist", channelname)
		}
		messagesKey = "messages/" + channelname[0] + ":"
	} else {
		messagesKey = "messages/general:"
	}

	if members := r.client.SMembers(r.ctx, messagesKey).Val(); len(members) != 0 {
		messages := make([]*types.ChatMessage, 0, len(members))
		for _, messageid := range members { // O(n^2)
			data := r.client.HGetAll(r.ctx, messageid).Val()
			if len(data) == 0 {
				log.Logger.Panic("Message %s not found", messageid)
			}

			message := &types.ChatMessage{}

			value := reflect.ValueOf(message).Elem()
			for i := 0; i < value.NumField(); i++ {
				fieldname := value.Type().Field(i).Name
				if fieldname == value.Type().Field(0).Name {
					contents := data[fieldname]
					buf := bytes.NewBuffer(make([]byte, 0, len(contents)))
					buf.WriteString(contents)
					value.Field(i).Set(reflect.ValueOf(buf))
					continue
				}
				value.Field(i).Set(reflect.ValueOf(data[fieldname]))
			}
			message = (*types.ChatMessage)(value.Addr().UnsafePointer())
			messages = append(messages, message)
		}
		return messages
	}
	return nil
}

func (r *redisBackend) GetChannels() []*types.Channel {
	r.RLock()
	defer r.RUnlock()

	members := r.client.SMembers(r.ctx, "channels:")
	if len(members.Val()) == 0 {
		return nil
	}

	channels := make([]*types.Channel, 0, len(members.Val()))

	for _, channelName := range members.Val() {
		channelHash := util.Sha256Checksum([]byte(channelName))

		data := r.client.HGetAll(r.ctx, channelHash)
		if len(data.Val()) == 0 {
			log.Logger.Panic("Channel %s was't found", channelName)
		}

		channel := &types.Channel{}

		value := reflect.ValueOf(channel).Elem()
		for i := 0; i < value.NumField(); i++ {
			fieldname := value.Type().Field(i).Name
			switch value.Field(i).Type() {
			case reflect.TypeOf(channel.Members):
			case reflect.TypeOf(channel.ChatHistory):
			default:
				value.Field(i).Set(reflect.ValueOf(data.Val()[fieldname]))
			}
		}
		channel = (*types.Channel)(value.Addr().UnsafePointer())
		channels = append(channels, channel)
	}
	return channels
}

func (r *redisBackend) GetParticipants() []*types.Participant {
	r.RLock()
	defer r.RUnlock()

	// TODO: Figure out how to use redis transactions in order to speed up the performance
	// (reduce the amount of calls to the redis server)
	members := r.client.SMembers(r.ctx, "participants:")
	if len(members.Val()) == 0 {
		return nil
	}

	participants := make([]*types.Participant, 0, len(members.Val()))

	for _, participantUsername := range members.Val() {
		participantHash := util.Sha256Checksum([]byte(participantUsername))

		data := r.client.HGetAll(r.ctx, participantHash)
		if len(data.Val()) == 0 {
			log.Logger.Panic("Participant %s not found", participantUsername)
		}

		participant := &types.Participant{}

		value := reflect.ValueOf(participant).Elem()
		for i := 0; i < value.NumField(); i++ {
			fieldname := value.Type().Field(i).Name
			value.Field(i).Set(reflect.ValueOf(data.Val()[fieldname]))
		}
		participant = (*types.Participant)(value.Addr().UnsafePointer())
		participants = append(participants, participant)
	}
	return participants
}
