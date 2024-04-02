package memory

import "time"

type MemoryBackend struct {
	Storage map[string]*map[string]string
}

func (mb *MemoryBackend) ContainsClient(identifier string) bool {
	return true
}

func (mb *MemoryBackend) RegisterClient(identifier string, ipAddress string, status string, joinedTime time.Time) bool {
	return true
}

func (mb *MemoryBackend) UpdateClient(identifier string, rest ...any) bool {
	return true
}

func (mb *MemoryBackend) GetParticipantsList() ([][3]string, error) {
	return nil, nil
}
