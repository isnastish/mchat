package session

import (
	"net"
	"strings"
	"sync"
	"time"
)

type ClientState int32

const (
	ClientState_Pending ClientState = iota + 1
	ClientState_Connected
	ClientState_Disconnected
)

type Client struct {
	conn     net.Conn
	ipAddr   string
	name     string
	joinTime time.Time
	state    ClientState
}

type ClientsMap struct {
	data   map[string]*Client
	mu     sync.Mutex
	_count int32
}

func stateStr(state ClientState) string {
	switch state {
	case ClientState_Connected:
		return "Online"
	case ClientState_Disconnected:
		return "Offile"
	case ClientState_Pending:
		return "Pending"
	default:
		return "Unknown"
	}
}

func (c *Client) _writeBytes(contents []byte, contentsSize int) (int, error) {
	var bWritten int
	for bWritten < contentsSize {
		n, err := c.conn.Write(contents[bWritten:])
		if err != nil {
			return bWritten, err
		}
		bWritten += n
	}
	return bWritten, nil
}

func newMap() *ClientsMap {
	return &ClientsMap{
		data: make(map[string]*Client),
		mu:   sync.Mutex{},
	}
}

func (m *ClientsMap) addClient(id string, c *Client) {
	m.mu.Lock()
	m.data[id] = c
	m.mu.Unlock()
}

func (m *ClientsMap) removeClient(id string) {
	m.mu.Lock()
	delete(m.data, id)
	m.mu.Unlock()
}

func (m *ClientsMap) size() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.data)
}

func (m *ClientsMap) existsWithState(id string, state ClientState) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	client, exists := m.data[id]
	return exists && client.state == state
}

func (m *ClientsMap) updateState(addr string, state ClientState) {
	m.mu.Lock()
	m.data[addr].state = state
	m.mu.Unlock()
}

func (m *ClientsMap) broadcastMessage(msg Message, excludes []string, recipients []string) (int, int) {
	var sentMessages int
	var droppedMessages int
	fMsg, fMsgSize := msg.Format()

	m.mu.Lock()
	if len(recipients) != 0 {
		for _, recipient := range recipients {
			recipient, exists := m.data[recipient]
			if exists && (recipient.state == ClientState_Connected ||
				recipient.state == ClientState_Pending) {
				bytesWritten, err := recipient._writeBytes(fMsg, fMsgSize)
				if err != nil || bytesWritten != fMsgSize {
					droppedMessages++
				} else {
					sentMessages++
				}
			}
		}
	} else {
		// O(n^2) Could we do better?
		for _, client := range m.data {
			if client.state == ClientState_Connected {
				for _, exclude := range excludes {
					if strings.Compare(client.name, exclude) != 0 {
						// NOTE: This might fail. Client could have already disconnected
						// while we were iterating over the map of all clients.
						// So we can log what happend and increment droppedMessages
						bWritten, err := client._writeBytes(fMsg, fMsgSize)
						if err != nil || bWritten != fMsgSize {
							// log.Error().Msgf("failed to write bytes to [%s], error %s", client.name, err.Error())
							droppedMessages++
						} else {
							sentMessages++
						}
						break
					}
				}
			}
		}
	}
	m.mu.Unlock()

	return sentMessages, droppedMessages
}

func (m *ClientsMap) getClients() map[string]string {
	var state string
	var result map[string]string

	m.mu.Lock()
	for _, client := range m.data {
		if client.state != ClientState_Pending {
			state = stateStr(client.state)
		} else {
			continue
		}
		result[client.name] = state
	}
	m.mu.Unlock()

	return result
}
