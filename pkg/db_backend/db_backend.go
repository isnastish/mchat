package dbbackend

import "time"

const (
	BackendType_Mysql = "mysql"
	BackendType_Redis = "redis"

	// In-memory storage should be used for local development, only.
	// All session's history should be stored in a local memory,
	// and freed at the end of a session.
	BackendType_Memory = "memory"
)

// A way to separate a header from actual messages
type ClientInfo struct {
	Name     string
	Header   map[string]string
	Messages map[string]string
}

// And then map[string]*map[string]string

type DatabaseBackend interface {
	HasClient(clientName string) bool
	RegisterClient(clientName string, ipAddress string, status string, joinedTime time.Time) error
	AddMessage(clientName string, sentTime time.Time, body [1024]byte)

	// A key for messages would be sender's id + time when the messages was sent.
	// We should introduce a way to persist messages somehow.
	GetClients() (map[string]*map[string]string, error)
}

// Client's info:
// id
// name
// ip_address
// status
// joinTime
// messagesSent
// messagesReceived
