package dbbackend

import "time"

const (
	BackendType_Mysql = iota + 1
	BackendType_Redis
	BackendType_Memory // for local development only
)

// In case we need to display the info somewhere in the future
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

// Rather than operating on clients, we should operate on sessions itself.
// clients cannot communicate without creating a session.
type DatabaseBackend interface {
	// Either pass the whole session to a function, or bit by bit.
	// Worth noting that passing the whole session might cause import cycle.

	// AddNewSession(name string, participants map[string]*Client, chatHistory []Message) // should we pass a session name?
	// UpdateSession()
	// DoesSessionExist()
	// GetSessionHistory()

	HasClient(clientName string) bool
	RegisterClient(clientName string, ipAddress string, status string, joinedTime time.Time) error
	AddMessage(clientName string, sentTime time.Time, body [1024]byte)
	GetClients() (map[string]*map[string]string, error)
}
