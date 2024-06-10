package dynamodb

import (
	"sync"

	_ "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	_ "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/isnastish/chat/pkg/backend"
	_ "github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/types"
	_ "github.com/isnastish/chat/pkg/utilities"
	_ "github.com/isnastish/chat/pkg/validation"
)

type dynamodbBackend struct {
	client *dynamodb.Client
	sync.RWMutex
}

func NewDynamodbBackend(config *backend.DynamodbConfig) (*dynamodbBackend, error) {
	return &dynamodbBackend{}, nil
}

func (d *dynamodbBackend) HasParticipant(username string) bool {
	return false
}

func (d *dynamodbBackend) RegisterParticipant(participant *types.Participant) {
}

func (d *dynamodbBackend) AuthParticipant(participant *types.Participant) bool {
	return false
}

func (d *dynamodbBackend) StoreMessage(message *types.ChatMessage) {

}

func (d *dynamodbBackend) HasChannel(channelname string) bool {
	return false
}

func (d *dynamodbBackend) RegisterChannel(channel *types.Channel) {
}

func (d *dynamodbBackend) DeleteChannel(channelname string) bool {
	return false
}

func (d *dynamodbBackend) GetChatHistory() []*types.ChatMessage {
	return nil
}

func (d *dynamodbBackend) GetChannelHistory(channelname string) []*types.ChatMessage {
	return nil
}

func (d *dynamodbBackend) GetChannels() []*types.Channel {
	return nil
}

func (d *dynamodbBackend) GetParticipants() []*types.Participant {
	return nil
}
