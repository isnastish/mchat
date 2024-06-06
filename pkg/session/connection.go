package session

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/types"
	"github.com/isnastish/chat/pkg/utilities"
)

type connectionState int8

const (
	pendingState   connectionState = 0
	connectedState connectionState = 0x1
)

var connStateTable []string

type connection struct {
	netConn     net.Conn
	ipAddr      string
	participant *types.Participant
	channel     *types.Channel
	timeout     time.Duration

	// TODO: Better names.
	idleConnChan chan struct{}
	abortChan    chan struct{}
	quitChan     chan struct{}

	state connectionState
}

type connectionMap struct {
	connections map[string]*connection
	mu          sync.RWMutex
}

func initConnStateTable() {
	connStateTable[pendingState] = "offline"
	connStateTable[connectedState] = "online"
}

func newConn(conn net.Conn, timeout time.Duration) *connection {
	return &connection{
		netConn:      conn,
		ipAddr:       conn.RemoteAddr().String(),
		participant:  &types.Participant{},
		channel:      &types.Channel{},
		timeout:      timeout,
		idleConnChan: make(chan struct{}),
		abortChan:    make(chan struct{}),
		quitChan:     make(chan struct{}),

		state: pendingState,
	}
}

func newConnectionMap() *connectionMap {
	initConnStateTable()
	return &connectionMap{
		connections: make(map[string]*connection),
	}
}

func (c *connection) matchState(state connectionState) bool {
	return c.state == state
}

// This function should be triggered only when the client was able to register or authenticate.
// The time when a participant tries to submit all the data is not taken into account.
func (c *connection) disconnectIfIdle() {
	timer := time.NewTimer(c.timeout)
	for {
		select {
		case <-timer.C:
			close(c.idleConnChan)
			c.netConn.Close()
			return
		case <-c.abortChan:
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(c.timeout)
		case <-c.quitChan:
			return
		}
	}
}

func (cm *connectionMap) _doesConnExist(connIpAddr string) bool {
	_, exists := cm.connections[connIpAddr]
	return exists
}

func (cm *connectionMap) addConn(conn *connection) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.connections[conn.ipAddr] = conn
}

func (cm *connectionMap) removeConn(connIpAddr string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm._doesConnExist(connIpAddr) {
		log.Logger.Panic("Connection {%s} doesn't exist", connIpAddr)
	}

	delete(cm.connections, connIpAddr)
}

func (cm *connectionMap) hasConnectedParticipant(username string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, conn := range cm.connections {
		if conn.matchState(connectedState) {
			if conn.participant.Username == username {
				return true
			}
		}
	}
	return false
}

func (cm *connectionMap) empty() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.connections) == 0
}

func (cm *connectionMap) markAsConnected(connIpAddr string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm._doesConnExist(connIpAddr) {
		log.Logger.Panic("Connection {%s} doesn't exist", connIpAddr)
	}

	cm.connections[connIpAddr].state = connectedState
}

// Pointers to interfaces: https://stackoverflow.com/questions/44370277/type-is-pointer-to-interface-not-interface-confusion
func (cm *connectionMap) broadcastMessage(message interface{}) int {
	var sentCount int

	cm.mu.RLock()
	defer cm.mu.Unlock()

	switch msg := message.(type) {
	case *types.ChatMessage:
		senderWasSkipped := false
		for _, conn := range cm.connections {
			if conn.matchState(connectedState) {
				if !senderWasSkipped && strings.EqualFold(conn.participant.Username, msg.Sender) {
					senderWasSkipped = true
					continue
				}

				n, err := util.WriteBytes(conn.netConn, msg.Contents)
				if err != nil || (n != msg.Contents.Len()) {
					log.Logger.Error("Failed to send a chat message to the participant: %s", conn.participant.Username)
				} else {
					sentCount++
				}
			}
		}

	case *types.SysMessage:
		if msg.Recipient != "" {
			conn, exists := cm.connections[msg.Recipient]
			if exists {
				n, err := util.WriteBytes(conn.netConn, msg.Contents)
				if err != nil || (n != msg.Contents.Len()) {
					log.Logger.Error("Failed to send a system message to the connection ip: %s", conn.ipAddr)
				} else {
					sentCount++
				}
			}
		} else {
			// A case where messages about participants leaving broadcasted to all the other connected participants
			for _, conn := range cm.connections {
				if conn.matchState(connectedState) {
					n, err := util.WriteBytes(conn.netConn, msg.Contents)
					if err != nil || (n != msg.Contents.Len()) {
						log.Logger.Error("Failed to send a system message to the participant: %s", conn.participant.Username)
					} else {
						sentCount++
					}
				}
			}
		}

	default:
		log.Logger.Panic("Invalid message type")
	}

	return sentCount
}
