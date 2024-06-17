package session

import (
	"bytes"
	"context"
	"io"
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
	netConn                net.Conn
	ipAddr                 string
	participant            *types.Participant
	channel                *types.Channel
	timeout                time.Duration
	ctx                    context.Context
	cancel                 context.CancelFunc
	abortConnectionTimeout chan struct{}
	state                  connectionState
}

// TODO: Add test for broadcast message deadline
type connectionMap struct {
	connections          map[string]*connection
	broadcastMsgDeadline time.Duration
	sync.RWMutex
}

func init() {
	connStateTable = make([]string, 2)
	connStateTable[pendingState] = "offline"
	connStateTable[connectedState] = "online"
}

func newConn(conn net.Conn, timeout time.Duration) *connection {
	ctx, cancel := context.WithCancel(context.Background())
	return &connection{
		netConn:                conn,
		ipAddr:                 conn.RemoteAddr().String(),
		participant:            &types.Participant{},
		channel:                &types.Channel{},
		timeout:                timeout,
		ctx:                    ctx,
		cancel:                 cancel,
		abortConnectionTimeout: make(chan struct{}),
		state:                  pendingState,
	}
}

func newConnectionMap() *connectionMap {
	return &connectionMap{
		connections:          make(map[string]*connection),
		broadcastMsgDeadline: 10 * time.Second,
	}
}

func (c *connection) matchState(state connectionState) bool {
	return c.state == state
}

func (c *connection) disconnectIfIdle() {
	timer := time.NewTimer(c.timeout)
	for {
		select {
		case <-timer.C:
			// The timer has fired, close the net connection manually,
			// and invoke the cancel() function in order to send a message to the client
			// insie a select statement in reader::read() procedure.
			c.netConn.Close()
			c.cancel()
			return
		case <-c.abortConnectionTimeout:
			// A signal to abort the timeout process was received, probably due to the client being active
			// (was able to send a message that the session received).
			// In this case we have to stop the current timer and reset it.
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(c.timeout)
		case <-c.ctx.Done():
			// A signal to unblock this procedure was received,
			// so we can exit gracefully without having go routine leaks.
			return
		}
	}
}

func (cm *connectionMap) _doesConnExist(connIpAddr string) bool {
	_, exists := cm.connections[connIpAddr]
	return exists
}

func (cm *connectionMap) addConn(conn *connection) {
	cm.Lock()
	defer cm.Unlock()
	cm.connections[conn.ipAddr] = conn
}

func (cm *connectionMap) removeConn(connIpAddr string) {
	cm.Lock()
	defer cm.Unlock()

	if !cm._doesConnExist(connIpAddr) {
		log.Logger.Panic("Connection {%s} doesn't exist", connIpAddr)
	}

	delete(cm.connections, connIpAddr)
}

func (cm *connectionMap) hasConnectedParticipant(username string) bool {
	cm.RLock()
	defer cm.RUnlock()

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
	cm.RLock()
	defer cm.RUnlock()
	return len(cm.connections) == 0
}

func (cm *connectionMap) markAsConnected(connIpAddr string) {
	cm.Lock()
	defer cm.Unlock()

	if !cm._doesConnExist(connIpAddr) {
		log.Logger.Panic("Connection {%s} doesn't exist", connIpAddr)
	}

	cm.connections[connIpAddr].state = connectedState
}

// Pointers to interfaces: https://stackoverflow.com/questions/44370277/type-is-pointer-to-interface-not-interface-confusion
func (cm *connectionMap) broadcastMessage(msg interface{}) int {
	var sentCount int

	cm.RLock()
	defer cm.RUnlock()

	switch msg := msg.(type) {
	case *types.ChatMessage:
		senderWasSkipped := false
		// Convert message into a canonical form, which includes the name of the sender and the time when the message was sent.
		canonChatMsg := bytes.NewBuffer([]byte(util.Fmtln("{%s:%s} %s", msg.Sender, msg.SentTime, msg.Contents.String())))
		for _, conn := range cm.connections {
			if conn.matchState(connectedState) {
				if !senderWasSkipped && strings.EqualFold(conn.participant.Username, msg.Sender) {
					senderWasSkipped = true
					continue
				}

				hitDeadline := cm.writeWithDeadline(conn, canonChatMsg)
				if hitDeadline {
					conn.cancel()
					return 0
				}
			}
		}

	case *types.SysMessage:
		if msg.Recipient != "" {
			conn, exists := cm.connections[msg.Recipient]
			if exists {
				hitDeadline := cm.writeWithDeadline(conn, msg.Contents)
				if hitDeadline {
					conn.cancel()
					return 0
				}
			}
		} else {
			// The case where messages about participants leaving broadcasted to all the other connected participants
			for _, conn := range cm.connections {
				if conn.matchState(connectedState) {
					hitDeadline := cm.writeWithDeadline(conn, msg.Contents)
					if hitDeadline {
						conn.cancel()
						return 0
					}
				}
			}
		}

	default:
		log.Logger.Panic("Invalid message type")
	}

	return sentCount
}

func (cm *connectionMap) writeWithDeadline(conn *connection, bytesBuffer *bytes.Buffer) bool {
	deadlineSig := make(chan struct{})
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(cm.broadcastMsgDeadline))
	defer cancel()
	go func() {
		select {
		case <-ctx.Done():
			conn.netConn.Close()
			close(deadlineSig)
		}
	}()

	// TODO: We have to put a connection into a disconnected state, otherwise the server doesn't know
	// that the connection was close and will try to send a message
	bytesSent, err := util.WriteBytesToConn(conn.netConn, bytesBuffer.Bytes(), bytesBuffer.Len())
	if err != nil && err != io.EOF {
		select {
		case <-deadlineSig:
			log.Logger.Error("Timeout sending a message %s", bytesBuffer.String())
		default:
			log.Logger.Error("Failed to send a system message %v", err)
		}
		return true
	} else if bytesSent != bytesBuffer.Len() {
		log.Logger.Error("Partially sent data, expected %d, sent %d, contents %s",
			bytesBuffer.Len(), bytesSent, bytesBuffer.String(),
		)
	} else {
		// sentCount++
	}
	return false
}
