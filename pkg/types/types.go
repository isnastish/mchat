// TODO: Try replacing msg param to BuildSysMsg with strings.Builder.
package types

import (
	"bytes"

	"github.com/isnastish/chat/pkg/utilities"
)

// NOTE: The sent time should be computed on the client itself rather than on the server when it sends a message,
// and the receive time should be computed when a client receives the message.
// Maybe we could compute the time on a session to simplify things.

type Participant struct {
	Username string
	Password string
	Email    string
	JoinTime string
}

type ChatMessage struct {
	Contents *bytes.Buffer
	Sender   string
	Channel  string
	SentTime string
}

type SysMessage struct {
	Contents  *bytes.Buffer
	Recipient string
	SentTime  string
}

type Channel struct {
	Name         string
	Desc         string
	Creator      string
	CreationDate string
	ChatHistory  []*ChatMessage
	Members      []string
}

// Helper function for building system messages.
func BuildSysMsg(msg string, recipients ...string) *SysMessage {
	var recipient string
	if len(recipients) > 0 {
		recipient = recipients[0]
	}

	return &SysMessage{
		Contents:  bytes.NewBuffer([]byte(msg)),
		Recipient: recipient,
		SentTime:  util.TimeNowStr(),
	}
}

// Helper function for building chat messages.
func BuildChatMsg(msg []byte, sender string, channels ...string) *ChatMessage {
	var channel string
	if len(channel) > 0 {
		channel = channels[0]
	}

	return &ChatMessage{
		Contents: bytes.NewBuffer(bytes.Clone(msg)),
		Sender:   sender,
		Channel:  channel,
		SentTime: util.TimeNowStr(),
	}
}
