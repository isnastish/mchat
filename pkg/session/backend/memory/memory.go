package memory

import (
	"fmt"
	"sync"
)

type SentMessage struct {
	message  []byte
	sentTime string
}

type ClientData struct {
	name           string
	passwordSha256 string
	sentMessages   []SentMessage
	connTime       string
}

type MemBackend struct {
	storage map[string]*ClientData
	mu      sync.Mutex
}

func NewClient(name string, passwordSha256 string, connTime string) *ClientData {
	return &ClientData{
		name:           name,
		passwordSha256: passwordSha256,
		connTime:       connTime,
		sentMessages:   make([]SentMessage, 0),
	}
}

func NewMemBackend() *MemBackend {
	return &MemBackend{
		storage: make(map[string]*ClientData),
	}
}

func (b *MemBackend) DoesClientExist(name string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, exists := b.storage[name]
	return exists
}

func (b *MemBackend) RegisterClient(clientname string, passwordSha256 string, connTime string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.DoesClientExist(clientname) {
		return fmt.Errorf("client with name {%s} already registered", clientname)
	}
	b.storage[clientname] = NewClient(clientname, passwordSha256, connTime)
	return nil
}

func (b *MemBackend) RegisterMessage(clientname string, sentTime string, message []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.DoesClientExist(clientname) {
		return fmt.Errorf("failed to register the message, client {%s} doesn't exist", clientname)
	}
	client := b.storage[clientname]
	client.sentMessages = append(client.sentMessages, SentMessage{
		message:  message,
		sentTime: sentTime,
	})

	return nil
}
