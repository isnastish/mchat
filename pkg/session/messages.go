package session

import (
	"fmt"
	t "time"
)

type ClientMessage struct {
	Contents     []byte
	SenderName   string
	SenderIpAddr string // TODO: Most likely we don't need an api address, it's not being used.
	SendTime     t.Time
}

// TODO: Think about better name
type SessionMessage struct {
	Recipient  *Client
	Contents   []byte
	SendTime   t.Time
	IgnoreTime bool
}

// TODO: Give more descriptive name
type GreetingMessage struct {
	SendTime t.Time
	Contents []byte
	Exclude  *Client
}

func NewClientMsg(senderIpAddr string, senderName string, contents []byte) *ClientMessage {
	return &ClientMessage{
		Contents:     contents,
		SenderName:   senderName,
		SenderIpAddr: senderIpAddr,
		SendTime:     t.Now(),
	}
}

func NewSessionMsg(recipient *Client, contents []byte, ignoreTime ...bool) *SessionMessage {
	var ignore bool

	if len(ignoreTime) > 0 {
		ignore = ignoreTime[0]
	}

	return &SessionMessage{
		Recipient:  recipient,
		Contents:   contents,
		SendTime:   t.Now(),
		IgnoreTime: ignore,
	}
}

func NewGreetingMsg(exclude *Client, contents []byte) *GreetingMessage {
	return &GreetingMessage{
		SendTime: t.Now(),
		Contents: contents,
		Exclude:  exclude,
	}
}

func (m *SessionMessage) Format() ([]byte, int) {
	if m.IgnoreTime {
		return m.Contents, len(m.Contents)
	}

	res := []byte(fmt.Sprintf("[%s] %s", m.SendTime.Format(t.DateTime), string(m.Contents)))
	size := len(res)

	return res, size
}

func (m *ClientMessage) Format() ([]byte, int) {
	res := []byte(fmt.Sprintf("[%s] [%s] %s", m.SendTime.Format(t.DateTime), m.SenderName, string(m.Contents)))
	size := len(res)

	return res, size
}

func (m *GreetingMessage) Format() ([]byte, int) {
	res := []byte(fmt.Sprintf("[%s] [%s] %s", m.SendTime.Format(t.DateTime), string(m.Contents)))
	size := len(res)

	return res, size
}
