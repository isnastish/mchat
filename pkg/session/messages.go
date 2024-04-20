package session

import (
	"fmt"
	"sync"
	t "time"
)

// TODO: Modify Greeting messages so we don't pass a pointer to a client to be excluded.
// The same applies to SessionMessage's Recipient
// And don't capitalize fields if they're not used by external packages.
// TODO: Remove all the client pointers to Client and only maintain strings.
// Make it as simple as possible.

type Message interface {
	Format() ([]byte, int)
}

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

type MessageHistory struct {
	messages []ClientMessage
	count    int
	cap      int
	mu       sync.Mutex
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
	res := []byte(fmt.Sprintf("[%s] %s", m.SendTime.Format(t.DateTime), string(m.Contents)))
	size := len(res)

	return res, size
}

func (h *MessageHistory) addMessage(msg *ClientMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.count >= h.cap {
		new_cap := max(h.cap+1, h.cap<<2)
		new_messages := make([]ClientMessage, new_cap)
		copy(new_messages, h.messages)
		h.messages = new_messages
		h.cap = new_cap
	}
	h.messages[h.count] = *msg
	h.count++
}

func (h *MessageHistory) size() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.count
}

func (h *MessageHistory) flush(out []ClientMessage) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return copy(out, h.messages)
}
