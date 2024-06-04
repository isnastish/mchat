package types

import (
	"bytes"
)

// NOTE: The sent time should be computed on the client itself rather than on the server when it sends a message,
// and the receive time should be computed when a client receives the message.
// Maybe we could compute the time on a session to simplify things.

type MessageKind int8

const (
	ChatMessageType MessageKind = 0x01
	SysMessageType  MessageKind = 0x02
)

type Participant struct {
	Username string
	Password string
	Email    string
	JoinTime string
}

type Message interface{}

type ChatMessage struct {
	Contents *bytes.Buffer
	Sender   string
	Channel  string
	SentTime string
}

type SysMessage struct {
	Contents    *bytes.Buffer
	ReceiveList []string
	SentTime    string
}

type Channel struct {
	Name         string
	Desc         string
	Creator      string
	CreationDate string
	ChatHistory  []*ChatMessage
	Members      []string
}

type RedisConfig struct {
}

type DynamodbConfig struct {
}
