package dbbackend

import "time"

const (
	BackendType_Mysql = iota + 1 // TODO(alx): Replace with DynamoDB
	BackendType_Redis
	BackendType_Memory
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

type DatabaseBackend interface {
	HasClient(name string) bool
	MatchClientPassword(name string, password string) bool
	RegisterNewClient(name string, addr string, status string, password string, connTime time.Time) bool
	AddMessage(name string, contents_ []byte, sentTime_ time.Time) bool
}
