package memory

import (
	lgr "github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/session"
)

type MemoryBackend struct {
	storage map[string]*session.Session
}

var log = lgr.NewLogger("debug")

func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{
		storage: make(map[string]*session.Session),
	}
}

func (mb *MemoryBackend) StoreSession(sessionId string, session *session.Session) bool {
	// assuming a session with this sessionId doesn't exist
	mb.storage[sessionId] = session
	log.Info().Msgf("session [%s] was saved", sessionId)

	return true
}

func (mb *MemoryBackend) DoesSessionExist(sessionId string) (*session.Session, bool) {
	session, exists := mb.storage[sessionId]
	return session, exists
}

func (mb *MemoryBackend) GetSessionHistory(sessionId string) ([]session.Message, bool) {
	session, exists := mb.storage[sessionId]
	if !exists {
		log.Warn().Msgf("session [%s] doesn't exist", sessionId)
		return nil, false
	}
	return session.MessageHistory, true
}
