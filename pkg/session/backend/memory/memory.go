package memory

import (
	lgr "github.com/isnastish/chat/pkg/logger"
	"strings"
	t "time"
)

type SentMessage struct {
	contents []byte
	sentTime t.Time
}

type ClientData struct {
	name           string
	addr           string // not neccessary, because it changes
	passwordSha256 string
	status         string // online | offline (not neccessary)
	sentMessages   []SentMessage
	connTime       t.Time
}

type MemBackend struct {
	storage map[string]*ClientData
}

var log = lgr.NewLogger("debug")

func NewClient(name_ string, addr_ string, password_ string, status_ string, connTime_ t.Time) *ClientData {
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

func (b *MemBackend) HasClient(name string) bool {
	_, exists := b.storage[name]
	return exists
}

func (b *MemBackend) MatchClientPassword(name string, password string) bool {
	log.Info().Msgf("matching a password [%s] for client [%s]", password, name)
	client, exists := b.storage[name]
	if exists {
		if strings.Compare(client.passwordSha256, password) == 0 {
			return true
		}
	}
	return false
}

func (b *MemBackend) RegisterNewClient(name string, addr string, status string, password string, connTime t.Time) bool {
	log.Info().Msgf("registering new client [%s]", name)
	b.storage[name] = NewClient(name, addr, status, password, connTime)
	return true
}

func (b *MemBackend) AddMessage(name string, contents_ []byte, sentTime_ t.Time) bool {
	log.Info().Msgf("message [%s] added to client [%s]", string(contents_), name)
	client, exists := b.storage[name]
	if exists {
		client.sentMessages = append(client.sentMessages, SentMessage{contents: contents_, sentTime: sentTime_})
		return true
	}
	return false
}
