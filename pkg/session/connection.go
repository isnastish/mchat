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
	Pending   connectionState = 0x1
	Connected connectionState = 0x2
)

var connectionStateTable = []string{
	"offline",
	"online",
}

type connection struct {
	netConn      net.Conn
	ipAddr       string
	state        connectionState
	participant  *types.Participant
	connTimeout  *time.Timer
	duration     time.Duration
	idleConnChan chan struct{}
	abortChan    chan struct{}
	quitChan     chan struct{}
}

type connectionMap struct {
	connections map[string]*connection
	mu          sync.Mutex
}

func newConnectionMap() *connectionMap {
	return &connectionMap{
		connections: make(map[string]*connection),
	}
}

func (c connection) isState(state connectionState) bool {
	return c.state == state
}

func (c *connection) disconnectIfIdle() {
	for {
		select {
		case <-c.connTimeout.C:
			close(c.idleConnChan)
			c.netConn.Close()
			return
		case <-c.abortChan:
			if !c.connTimeout.Stop() {
				<-c.connTimeout.C
			}
			c.connTimeout.Reset(c.duration)
		case <-c.quitChan:
			return
		}
	}
}

func (cm *connectionMap) addConn(conn *connection) {
	cm.mu.Lock()
	cm.connections[conn.ipAddr] = conn
	cm.mu.Unlock()
}

func (cm *connectionMap) removeConn(connIpAddr string) {
	cm.mu.Lock()
	delete(cm.connections, connIpAddr)
	cm.mu.Unlock()
}

func (cm *connectionMap) totalCount() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return len(cm.connections)
}

func (cm *connectionMap) assignParticipant(connIpAddr string, participant *types.Participant) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	conn, exists := cm.connections[connIpAddr]
	if !exists {
		log.Logger.Info("Connection ip: %s doesn't exist", connIpAddr)
	}

	conn.participant = participant
	conn.state = Connected
}

// Pointers to interfaces: https://stackoverflow.com/questions/44370277/type-is-pointer-to-interface-not-interface-confusion
func (cm *connectionMap) broadcastMessage(message types.Message, kind types.MessageKind) int {
	var sentCount int

	cm.mu.Lock()
	if kind == types.ChatMessageType {
		senderWasSkipped := false
		msg := message.(types.ChatMessage)
		for _, conn := range cm.connections {
			if conn.isState(Connected) {
				if !senderWasSkipped && strings.EqualFold(conn.participant.Username, msg.Sender) {
					senderWasSkipped = true
					continue
				}

				n, err := utilities.WriteBytes(conn.netConn, msg.Contents)
				if err != nil || (n != msg.Contents.Len()) {
					log.Logger.Error("Failed to send a chat message to the participant: %s", conn.participant.Username)
				} else {
					sentCount++
				}
			}
		}
	} else {
		msg := message.(types.SysMessage)
		if len(msg.ReceiveList) != 0 {
			for _, receiver := range msg.ReceiveList {
				conn, exists := cm.connections[receiver]
				if exists {
					n, err := utilities.WriteBytes(conn.netConn, msg.Contents)
					if err != nil || (n != msg.Contents.Len()) {
						log.Logger.Error("Failed to send a system message to the connection ip: %s", conn.ipAddr)
					} else {
						sentCount++
					}
				}
			}
		} else {
			// A case where messages about participants leaving broadcasted to all the other connected participants
			for _, conn := range cm.connections {
				if conn.isState(Connected) {
					n, err := utilities.WriteBytes(conn.netConn, msg.Contents)
					if err != nil || (n != msg.Contents.Len()) {
						log.Logger.Error("Failed to send a system message to the participant: %s", conn.participant.Username)
					} else {
						sentCount++
					}
				}
			}
		}
	}
	cm.mu.Unlock()

	return sentCount
}
