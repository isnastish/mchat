// TODO: Add a capability for deleting single messages
// and deleting all the messages in a channel and in a general chat.
package backend

import (
	"github.com/isnastish/chat/pkg/types"
)

type BackendType int8

const (
	BackendTypeDynamodb BackendType = 0
	BackendTypeRedis    BackendType = 0x1
	BackendTypeMemory   BackendType = 0x2
)

var BackendTypes [3]string

func init() {
	BackendTypes[BackendTypeDynamodb] = "dynamodb"
	BackendTypes[BackendTypeRedis] = "redis"
	BackendTypes[BackendTypeMemory] = "memory"
}

type Backend interface {
	HasParticipant(username string) bool
	RegisterParticipant(participant *types.Participant)
	AuthParticipant(participant *types.Participant) bool
	StoreMessage(message *types.ChatMessage)
	HasChannel(channelname string) bool
	RegisterChannel(channel *types.Channel)
	DeleteChannel(channelname string) bool
	GetChatHistory(channelname ...string) []*types.ChatMessage
	GetChannels() []*types.Channel
	GetParticipants() []*types.Participant
}

type RedisConfig struct {
	Password string
	Username string
	Endpoint string
}

type DynamodbConfig struct {
}

type Config struct {
	BackendType    BackendType
	RedisConfig    *RedisConfig
	DynamodbConfig *DynamodbConfig
}
