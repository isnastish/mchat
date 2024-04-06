package backend

import (
	"github.com/isnastish/chat/pkg/session"
)

const (
	BackendType_Mysql = iota + 1
	BackendType_Redis
	BackendType_Memory // for local development only
	BackendType_DynamoDb
)

func BackendTypeStr(backendType int) string {
	switch backendType {
	case BackendType_Mysql:
		return "mysql"
	case BackendType_Redis:
		return "redis"
	case BackendType_Memory:
		return "memory"
	default:
		return "undefined"
	}
}

type SessionStorageBackend interface {
	StoreSession(sessionId string, session *session.Session) bool
	DoesSessionExist(sessionId string) (*session.Session, bool)
	GetSessionHistory(sessionId string) ([]session.Message, bool)
	// ListSessions()
	// UpdateSession()
}
