// NOTE: Memory backend should only be used for local development.
// so we don't rely on any data storage.
// That implies that all the sessione's history will be erased by the end of programm execution,
// or an alternative would be to upload it to a file.

package memory

import "time"

type MemoryBackend struct {
	storage map[string]*map[string]string
}

func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{
		storage: make(map[string]*map[string]string),
	}
}

func (mb *MemoryBackend) HasClient(name string) bool {
	_, exists := mb.storage[name]
	return exists
}

func (mb *MemoryBackend) RegisterClient(name string, ipAddress string, status string, joinedTime time.Time) error {
	clientInfo := make(map[string]string)
	clientInfo["ip_address"] = ipAddress
	clientInfo["status"] = status
	clientInfo["joinedTime"] = joinedTime.Format(time.DateTime)

	mb.storage[name] = &clientInfo

	return nil
}

func (mb *MemoryBackend) AddMessage(clientName string, sentTime time.Time, body [1024]byte) {

}

func (mb *MemoryBackend) GetClients() (map[string]*map[string]string, error) {
	return nil, nil
}
