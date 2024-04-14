package session

import (
	"fmt"
	t "time"
)

// Client messages are broadcasted to all the connected clients in a current session,
// that's why they don't have a specific receiver, as SessionMessage instance.
type ClientMessage struct {
	senderIpAddr string // ?
	senderName   string
	sendTime     t.Time
	contents     []byte
}

func NewClientMsg(senderIpAddr string, senderName string, contents []byte) *ClientMessage {
	return &ClientMessage{
		senderIpAddr: senderIpAddr,
		senderName:   senderName,
		sendTime:     t.Now(),
		contents:     contents,
	}
}

func (m *ClientMessage) format() string {
	return fmt.Sprintf("%s [%s]: %s",
		m.sendTime.Format(t.DateTime),
		m.senderName,
		string(m.contents),
	)
}

type SessionMessage struct {
	receiver   string
	sendTime   t.Time
	contents   []byte
	ignoreTime bool
}

func NewSessionMsg(receiver string, contents []byte, ignoreTime ...bool) *SessionMessage {
	var ignore bool
	if len(ignoreTime) > 0 {
		ignore = ignoreTime[0]
	}

	return &SessionMessage{
		receiver:   receiver,
		contents:   contents,
		sendTime:   t.Now(),
		ignoreTime: ignore,
	}
}

func (m *SessionMessage) format() []byte {
	if m.ignoreTime {
		return m.contents
	}
	return []byte(fmt.Sprintf("[%s] %s", m.sendTime.Format(t.DateTime), string(m.contents)))
}

type GreetingMessage struct {
	sendTime t.Time
	contents []byte
	exclude  string
	// ignoreTime bool
}

func NewGreetingMsg(contents []byte, exclude string) *GreetingMessage {
	return &GreetingMessage{
		sendTime: t.Now(),
		contents: contents,
		exclude:  exclude,
	}
}

// func (m *GreetingMessage) format() []byte {
// 	if m.ignoreTime {
// 		return m.contents
// 	}
// 	return []byte(fmt.Sprintf("[%s] %s", m.sendTime.Format(t.DateTime), string(m.contents)))
// }
