package types

import (
	"bytes"
	"time"
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
	JoinTime time.Time
}

type Message interface{}

type ChatMessage struct {
	Contents *bytes.Buffer
	Sender   string
	Channel  string
	Time     time.Time
}

type SysMessage struct {
	Contents    *bytes.Buffer
	ReceiveList []string
	Time        time.Time
}

type Channel struct {
	Name         string
	Desc         string
	Creator      string
	CreationDate time.Time
	ChatHistory  []*ChatMessage
	Members      []*Participant
}

type RedisConfig struct {
}

type DynamodbConfig struct {
}