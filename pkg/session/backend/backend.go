package dbbackend

type BackendType int32

const (
	BackendType_DynamoDB = iota + 1
	BackendType_Redis
	BackendType_Memory
)

func BackendTypeStr(backendType BackendType) string {
	switch backendType {
	case BackendType_DynamoDB:
		return "dynamodb"
	case BackendType_Redis:
		return "redis"
	case BackendType_Memory:
		return "memory"
	default:
		return "unknown backend"
	}
}

type DatabaseBackend interface {
	DoesClientExist(name string) bool
	AuthClient(name string, passwordSha256 string) (bool, error)
	RegisterClient(name string, passwordSha256 string, connTime string) error
	RegisterMessage(senderName string, sentTime string, message []byte) error

	// channel
	// RegisterChannel
}
