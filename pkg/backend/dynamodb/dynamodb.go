package dynamodb

import (
	"sync"

	_ "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	_ "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/types"
	"github.com/isnastish/chat/pkg/utilities"
	"github.com/isnastish/chat/pkg/validation"
)

type DynamodbBackend struct {
	client *dynamodb.Client
	mu     sync.Mutex
}

// HasParticipant(username string) bool
// RegisterParticipant(participant *types.Participant)
// AuthParticipant(participant *types.Participant) bool
// StoreMessage(message *types.ChatMessage)
// HasChannel(channelname string) bool
// RegisterChannel(channel *types.Channel)
// DeleteChannel(channelname string) bool
// GetChatHistory() []*types.ChatMessage
// GetChannelHistory(channelname string) []*types.ChatMessage
// GetChannels() []*types.Channel
// GetParticipantList() []*types.Participant

func NewDynamodbBackend() (*DynamodbBackend, error) {
	return &DynamodbBackend{}, nil
}

func (d *DynamodbBackend) HasParticipant(username string) bool {
	return false
}

func (d *DynamodbBackend) RegisterParticipant(participant *types.Participant) {
	d.mu.Lock()

	// TODO: Check whether a participant already exists first

	passwordHash := utilities.Sha256Checksum([]byte(participant.Password))
	if validation.ValidatePasswordSha256(passwordHash) {
		log.Logger.Panic("Failed to register participant %s. Password validation failed", participant.Username)
	}

	d.mu.Unlock()
}

func (d *DynamodbBackend) AuthParticipant(participant *types.Participant) bool {
	return false
}

func (d *DynamodbBackend) StoreMessage(message *types.ChatMessage) {

}

func (d *DynamodbBackend) HasChannel(channelname string) bool {
	return false
}

func (d *DynamodbBackend) RegisterChannel(channel *types.Channel) {
}

func (d *DynamodbBackend) DeleteChannel(channelname string) bool {
	return false
}

func (d *DynamodbBackend) GetChatHistory() []*types.ChatMessage {
	return nil
}

func (d *DynamodbBackend) GetChannelHistory(channelname string) []*types.ChatMessage {
	return nil
}

func (d *DynamodbBackend) GetChannels() []*types.Channel {
	return nil
}

func (d *DynamodbBackend) GetParticipants() []*types.Participant {
	return nil
}
