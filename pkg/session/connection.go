package session

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	backend "github.com/isnastish/chat/pkg/session/backend"
)

const (
	RegisterParticipant     = 0x01
	AuthenticateParticipant = 0x02
	CreateChannel           = 0x03
	SelectChannel           = 0x04
	Exit                    = 0x05
)

var menuOptionsTable = []string{
	"Register",          // Register a new participant.
	"Log in",            // Authenticate an already registered participant.
	"Create channel",    // Create a new channel.
	"Select channels",   // Select a channel for writing messages.
	"List participants", // List all participants
	"Exit",              // Exit the sesssion.
}

type ConnectionState int32

const (
	Pending   ConnectionState = 0x1
	Connected ConnectionState = 0x2
)

var connectionStateTable = []string{
	"offline",
	"online",
}

var usernameMessageContents = []byte("username: ")
var passwordMessageContents = []byte("password: ")
var emailAddressMessageContents = []byte("email address: ")
var channelsNameMessageContents = []byte("channel's name: ")
var channelsDescMessageContents = []byte("channel's desc: ")

type ReaderState int32
type ReaderSubstate int32

// TODO(alx): Combine listing channels and selecting channels all together.
// SelectChannels will list all available channels and wait for the input.
const (
	ProcessingMenu            ReaderState = 0x01
	RegisteringNewParticipant ReaderState = 0x02
	AuthenticatingParticipant ReaderState = 0x03
	AcceptingMessages         ReaderState = 0x04
	CreatingNewChannel        ReaderState = 0x05
	// This state should list all available channels
	// and wait for input from the participant.
	SelectingChannel ReaderState = 0x06

	Disconnecting ReaderState = 0x07
)

// TODO(alx): You bitwise operations as an optimization
// ProcessingName = (1 << 1)
const (
	NotSet                             ReaderSubstate = 0x0
	ProcessingName                     ReaderSubstate = 0x01
	ProcessingParticipantsPassword     ReaderSubstate = 0x02
	ProcessingParticipantsEmailAddress ReaderSubstate = 0x03
	ProcessingChannelsDesc             ReaderSubstate = 0x04
)

type Connection struct {
	conn   net.Conn
	ipAddr string

	// TODO(alx): Document in the architecture.md file that a participant only
	// present when the state is Connect.
	state       ConnectionState
	participant *backend.Participant
}

func (c *Connection) isState(state ConnectionState) bool {
	return c.state == state
}

type ConnectionMap struct {
	connections map[string]*Connection
	mu          sync.Mutex
}

func newConnectionMap() *ConnectionMap {
	return &ConnectionMap{
		connections: make(map[string]*Connection),
	}
}

func (connMap *ConnectionMap) addConn(conn *Connection) {
	connMap.mu.Lock()
	connMap.connections[conn.ipAddr] = conn
	connMap.mu.Unlock()
}

func (connMap *ConnectionMap) removeConn(connIpAddr string) {
	connMap.mu.Lock()
	delete(connMap.connections, connIpAddr)
	connMap.mu.Unlock()
}

// Return the amount of connected participants
func (connMap *ConnectionMap) count() int {
	connMap.mu.Lock()
	defer connMap.mu.Unlock()
	return len(connMap.connections)
}

func (connMap *ConnectionMap) assignParticipant(connIpAddr string, participant *backend.Participant) {
	connMap.mu.Lock()
	defer connMap.mu.Unlock()

	conn, exists := connMap.connections[connIpAddr]
	if !exists {
		panic(fmt.Sprintf("connection with ip address {%s} doesn't exist", connIpAddr))
	}

	conn.participant = participant
	conn.state = Connected // When a participant is inserted its state is set to Connected as a side-effect
}

func (connMap *ConnectionMap) isParticipantConnected(participantName string) bool {
	connMap.mu.Lock()
	defer connMap.mu.Unlock()

	// not O(1) anymore since a connection map is a mapping from an ip address
	// to a connection, but a participant's name is provided instead,
	// so we cannot make a look up.
	for _, conn := range connMap.connections {
		if conn.isState(Connected) {
			if conn.participant.Name == participantName {
				return true
			}
		}
	}

	return false
}

type Reader struct {
	// A connection to read from
	conn   net.Conn
	ipAddr string

	// Reader's state
	state    ReaderState
	substate ReaderSubstate

	// If a participant was idle for too long, we disconnect it
	// by closing the connection and closing idleConnectionsCh channel
	// to indicate that the connection was closed intentionally by us.
	// If a message was sent, we have to send an abort signal,
	// indicating that a participant is active and there is no need to disconect it.
	// connection's timeout is reset in that case.
	// If an error occurs or a client closed the connection while reading bytes from a connection,
	// a signal sent onto errorSignalCh channel to unblock disconnectIfIdle function.
	connectionTimeout *time.Timer
	timeoutDuration   time.Duration

	idleConnectionsCh chan struct{}
	abortSignalCh     chan struct{}
	quitSignalCh      chan struct{}

	// Buffer for storing bytes read from a connection.
	buffer []byte

	// TODO(alx): Replace string(s) with []byte arrays?

	// Accumulated participant's data.
	participantsName           string
	participantsPasswordSha256 string
	participantsEmailAddress   string

	// Set to true if a participant was auathenticated or registered.
	wasAuthenticated bool

	// Current channel selected by the user
	// When a new channel is created, the user is automatically switched to that channel instead
	curChannel   backend.Channel
	channelIsSet bool
}

func newReader(conn net.Conn, connIpAddr string, connectionTimeout time.Duration) *Reader {
	return &Reader{
		conn:              conn,
		ipAddr:            conn.LocalAddr().String(),
		state:             ProcessingMenu,
		connectionTimeout: time.NewTimer(connectionTimeout),
		timeoutDuration:   connectionTimeout,
		idleConnectionsCh: make(chan struct{}),
		abortSignalCh:     make(chan struct{}),
		quitSignalCh:      make(chan struct{}),
		buffer:            make([]byte, 0, 1024),
	}
}

func (reader *Reader) isState(state ReaderState) bool {
	return reader.state == state
}

func (reader *Reader) isSubstate(substate ReaderSubstate) bool {
	return reader.substate == substate
}

func (reader *Reader) disconnectIfIdle() {
	for {
		select {
		case <-reader.connectionTimeout.C:
			close(reader.idleConnectionsCh)
			reader.conn.Close()
			return
		case <-reader.abortSignalCh:
			if !reader.connectionTimeout.Stop() {
				<-reader.connectionTimeout.C
			}
			reader.connectionTimeout.Reset(reader.timeoutDuration)
		case <-reader.quitSignalCh:
			// unblock this procedure
			return
		}
	}
}

type ReadResult struct {
	bytesRead int32
	asBytes   []byte
	asStr     string
	err       error
}

func (reader *Reader) read() ReadResult {
	bytesRead, err := reader.conn.Read(reader.buffer)
	str := strings.Trim(string(reader.buffer[:bytesRead]), " \\t\\r\\n\\v\\f")

	return ReadResult{
		bytesRead: int32(bytesRead),
		asBytes:   reader.buffer[:bytesRead],
		asStr:     str,
		err:       err,
	}
}

func (connMap *ConnectionMap) broadcastParticipantMessage(message *backend.ParticipantMessage) (int32, int32) {
	var droppedMessageCount int32
	var sentMessageCount int32

	formatedMessageContents := []byte(fmt.Sprintf(
		"[%s:%s]  %s",
		message.Sender,
		message.Time,
		message.Contents,
	))

	formatedMessageContentsLenght := len(formatedMessageContents)
	senderWasSkipped := false

	// O(n^2) Could we do better?
	connMap.mu.Lock()
	for _, connection := range connMap.connections {
		if connection.isState(Connected) {
			if !senderWasSkipped && strings.EqualFold(connection.participant.Name, message.Sender) {
				senderWasSkipped = true
				continue
			}

			// Messasges won't be sent to connections with a Pending state.
			bytesWritten, err := connection.writeBytes(formatedMessageContents, formatedMessageContentsLenght)
			if err != nil || (bytesWritten != formatedMessageContentsLenght) {
				droppedMessageCount++
			} else {
				sentMessageCount++
			}
		}
	}
	connMap.mu.Unlock()

	return droppedMessageCount, sentMessageCount
}

// excludeParticipantsList and receiveParticipantsList are mutually exclusive.
func (connMap *ConnectionMap) broadcastSystemMessage(message *backend.SystemMessage) (int32, int32) {
	var droppedMessageCount int32
	var sentMessageCount int32

	formatedMessageContents := []byte(fmt.Sprintf(
		"[%s]  %s",
		message.Time,
		message.Contents,
	))

	formatedMessageContentsLenght := len(formatedMessageContents)

	connMap.mu.Lock()
	if len(message.ReceiveList) != 0 {
		for _, receiver := range message.ReceiveList {
			connection, exists := connMap.connections[receiver]
			if exists {
				bytesWritten, err := connection.writeBytes(formatedMessageContents, formatedMessageContentsLenght)
				if err != nil || (bytesWritten != formatedMessageContentsLenght) {
					droppedMessageCount++
				} else {
					sentMessageCount++
				}
			}
		}
	} else {
		for _, connection := range connMap.connections {
			bytesWritten, err := connection.writeBytes(formatedMessageContents, formatedMessageContentsLenght)
			if err != nil || (bytesWritten != formatedMessageContentsLenght) {
				droppedMessageCount++
			} else {
				sentMessageCount++
			}
		}
	}
	connMap.mu.Unlock()

	return droppedMessageCount, sentMessageCount
}

func (c *Connection) writeBytes(contents []byte, contentsSize int) (int, error) {
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
