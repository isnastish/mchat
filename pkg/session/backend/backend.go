package dbbackend

import "time"

type BackendType int32

const (
	BackendTypeDynamoDB = iota + 1
	BackendTypeRedis
	BackendTypeMemory
)

func BackendTypeStr(backendType BackendType) string {
	switch backendType {
	case BackendTypeDynamoDB:
		return "dynamodb"
	case BackendTypeRedis:
		return "redis"
	case BackendTypeMemory:
		return "memory"
	default:
		return "unknown backend"
	}
}

type MessageType int32

const (
	MessageTypeParticipant MessageType = 0x01
	MessageTypeSystem
)

type Participant struct {
	Name           string
	PasswordSha256 string
	EmailAddress   string
	JoinTime       string
}

type IMessage interface{}

type ParticipantMessage struct {
	Contents []byte
	Sender   string
	Time     string
	Channel  string
}

type SystemMessage struct {
	Contents    []byte
	ReceiveList []string
	Time        string
}

func MakeSystemMessage(contents []byte, receiveList []string) *SystemMessage {
	return &SystemMessage{
		Contents:    contents,
		ReceiveList: receiveList,
		Time:        time.Now().Format(time.DateTime),
	}
}

func MakeParticipantMessage(contents []byte, sender string, channel ...string) *ParticipantMessage {
	m := &ParticipantMessage{
		Contents: contents,
		Sender:   sender,
		Time:     time.Now().Format(time.DateTime),
	}
	if len(channel) > 0 {
		m.Channel = channel[0]
	}
	return m
}

type Channel struct {
	Name         string
	Desc         string
	CreationDate string
	ChatHistory  []*ParticipantMessage
	Participants []*Participant
}

// NOTE(alx): All of these function has to be replaced with RPS(s)
// and the backend should be implemented as a separate service.
type Backend interface {
	// Check whether a participant with a given name exists.
	// Returns true if does, false otherwise.
	HasParticipant(username string) bool

	// Register a new participant with a unique username.
	// password is hashed using sha256 algorithm.
	RegisterParticipant(username string, passwordShaw256 string, emailAddress string)

	// Authenticate an already registered participant.
	// If username or password is incorrect, returns false.
	// A participant can either be authenticated by a name or by its email address.
	AuthParticipant(username string, passwordSha256 string) bool

	// Store a message in a backend storage.
	// If a message has a channel that it belongs to, it won't be displayed in a general chat history,
	// only in channel's history.
	StoreMessage(senderName string, sentTime string, message []byte, channelName ...string)

	// Check whether a channel with `channelName` exists.
	// Returns true if does, false otherwise.
	HasChannel(channelName string) bool

	// Register a new channel where clients can exchange messages.
	// All messages exchanged in a particular channel will be visible there only,
	// and not in a general chat.
	// Each channel has an owner, who reservs the right to give a permission for other participants to join.
	// *In order to acquire a permission, it shold be requested.
	RegisterChannel(channelName string, desc string, ownerName string)

	// Delete a channel if exists.
	DeleteChannel(channelName string) bool

	// Returns history of all the messages sent in a general chat.
	GetChatHistory(channelName ...string) []*ParticipantMessage

	// Returns all created channels.
	GetChannels() []*Channel

	// Return a list of all participants
	GetParticipantList() []*Participant
}
