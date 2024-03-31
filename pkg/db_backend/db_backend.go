package dbbackend

import "time"

const (
	BackendType_Mysql  = "mysql"
	BackendType_Redis  = "redis"
	BackendType_Memory = "memory"
)

type DatabaseBackend interface {
	ContainsClient(identifier string) bool
	RegisterClient(identifier string, ipAddress string, status string, joinedTime time.Time) bool
	UpdateClient(identifier string, rest ...any) bool
	GetParticipantsList() ([][3]string, error) // the return type has to be reconsidered
}
