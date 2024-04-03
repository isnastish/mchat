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

type DatabaseBackend interface {
	HasClient(clientName string) bool
	RegisterClient(clientName string, ipAddress string, status string, joinedTime time.Time) error
	AddMessage(clientName string, sentTime time.Time, body [1024]byte)
	GetClients() (map[string]*map[string]string, error)
}
