package memory

import (
	_ "github.com/isnastish/chat/pkg/logger"
	"strings"
	"sync"
	"time"
)

type SentMessage struct {
	contents []byte
	sentTime time.Time
}

type ClientData struct {
	name           string
	addr           string // not neccessary, because it changes
	passwordSha256 string
	status         string // online | offline (not neccessary)
	sentMessages   []SentMessage
	connTime       time.Time
}

type MemBackend struct {
	storage map[string]*ClientData
	mu      sync.Mutex
}

func NewClient(name_ string, addr_ string, password_ string, status_ string, connTime_ time.Time) *ClientData {
	return &ClientData{
		name:           name_,
		addr:           addr_,
		passwordSha256: password_,
		status:         status_,
		connTime:       connTime_,
		sentMessages:   make([]SentMessage, 0),
	}
}

func NewMemBackend() *MemBackend {
	return &MemBackend{
		storage: make(map[string]*ClientData),
	}
}

func (b *MemBackend) HasClient(id string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, exists := b.storage[id]
	return exists
}

func (b *MemBackend) MatchClientPassword(id string, password string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	client, exists := b.storage[id]
	return exists && strings.Compare(client.passwordSha256, password) == 0
}

func (b *MemBackend) RegisterNewClient(id string, addr string, status string, password string, connTime time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.storage[id] = NewClient(id, addr, status, password, connTime)
	return true
}

func (b *MemBackend) AddMessage(id string, contents_ []byte, sentTime_ time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	client, exists := b.storage[id]
	if exists {
		client.sentMessages = append(client.sentMessages, SentMessage{contents: contents_, sentTime: sentTime_})
		return true
	}
	return false
}
