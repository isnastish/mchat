package dynamodb

import (
	"sync"

	_ "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	_ "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	backend "github.com/isnastish/chat/pkg/session/backend"
)

type DynamoDBBackend struct {
	dynamoDbClient *dynamodb.Client
	mu             sync.Mutex
}

func NewBackend() *DynamoDBBackend {
	return &DynamoDBBackend{}
}

func (b *DynamoDBBackend) HasParticipant(username string) bool {
	return false
}

func (b *DynamoDBBackend) RegisterParticipant(username string, passwordShaw256 string, emailAddress string) {

}

func (b *DynamoDBBackend) AuthenticateParticipant(username string, passwordSha256 string) bool {
	return false
}

func (b *DynamoDBBackend) StoreMessage(senderName string, sentTime string, contents []byte, channelName ...string) {

}

func (b *DynamoDBBackend) HasChannel(channelName string) bool {
	return false
}

func (b *DynamoDBBackend) RegisterChannel(channelName string, desc string, ownerName string) {
}

func (b *DynamoDBBackend) DeleteChannel(channelName string) bool {
	return false
}

func (b *DynamoDBBackend) GetChatHistory(channelName ...string) []*backend.ParticipantMessage {
	return nil
}

func (b *DynamoDBBackend) GetChannelHistory(channelName string) {

}

func (b *DynamoDBBackend) GetChannels() []*backend.Channel {
	return nil
}

func (b *DynamoDBBackend) GetParticipantList() []*backend.Participant {
	return nil
}
