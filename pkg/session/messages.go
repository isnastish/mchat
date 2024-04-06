package session

import (
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

func newClientMsg(senderIpAddr string, senderName string, contents []byte) *ClientMessage {
	return &ClientMessage{
		senderIpAddr: senderIpAddr,
		senderName:   senderName,
		sendTime:     t.Now(),
		contents:     contents,
	}
}

type SessionMessage struct {
	receiver string
	sendTime t.Time // string?
	contents []byte
}

func newSessionMsg(receiver string, contents []byte) *SessionMessage {
	return &SessionMessage{
		receiver: receiver,
		contents: contents,
		sendTime: t.Now(),
	}
}

type GreetingMessage struct {
	sendTime t.Time
	contents []byte
	exclude  string
}

func newGreetingMsg(contents []byte, exclude string) *GreetingMessage {
	return &GreetingMessage{
		sendTime: t.Now(),
		contents: contents,
		exclude:  exclude,
	}
}
