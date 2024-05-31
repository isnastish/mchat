package session

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	backend "github.com/isnastish/chat/pkg/session/backend"
)

type MenuOptionType int8

const (
	RegisterParticipant     MenuOptionType = 0x01
	AuthenticateParticipant MenuOptionType = 0x02
	CreateChannel           MenuOptionType = 0x03
	SelectChannel           MenuOptionType = 0x04
	Exit                    MenuOptionType = 0x05
)

var menuOptionsTable = []string{
	"Register",          // Register a new participant.
	"Log in",            // Authenticate an already registered participant.
	"Create channel",    // Create a new channel.
	"Select channels",   // Select a channel for writing messages.
	"List participants", // List all participants
	"Exit",              // Exit the sesssion.
}

type ConnectionState int8

const (
	Pending   ConnectionState = 0x1
	Connected ConnectionState = 0x2
)

var connectionStateTable = []string{
	"offline",
	"online",
}

var menuMessageHeader = []byte("options:\r\n")
var channelsMessageHeader = []byte("channels:\r\n")
var participantListMessageHeader = []byte("participants:\r\n")
var usernameMessageContents = []byte("username: ")
var passwordMessageContents = []byte("password: ")
var emailAddressMessageContents = []byte("email address: ")
var channelsNameMessageContents = []byte("channel's name: ")
var channelsDescMessageContents = []byte("channel's desc: ")
var passwordValidationFailedMessageContents = []byte(CR("password validation failed"))
var emailAddressValidationFailedMessageContents = []byte(CR("email address validation failed"))
var usernameValidationFailedMessageContents = []byte(CR("username validation failed"))

type ReaderState int8
type ReaderSubstate int8

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

	ProcessingNumber ReaderSubstate = 0x05
)

// type State struct {
// 	state     ReaderState
// 	substate  ReaderSubstate
// 	validated bool
// }

// In most of the cases one state is not enought, we need an additional data to determine which state transition to make.
// var reader_transition_table = map[State]State{
// 	State{state: ProcessingMenu, substate: NotSet}: State{state: RegisteringNewParticipant, substate: ProcessingName},
// 	State{state: ProcessingMenu, substate: NotSet}: State{state: AuthenticatingParticipant, substate: ProcessingName},
// 	State{state: ProcessingMenu, substate: NotSet}: State{state: CreatingNewChannel, substate: ProcessingName},
// 	State{state: ProcessingMenu, substate: NotSet}: State{state: SelectingChannel, substate: ProcessingNumber},
// 	State{state: ProcessingMenu, substate: NotSet}: State{state: Disconnecting, substate: NotSet},

// 	State{state: RegisteringNewParticipant, substate: ProcessingName}:                                   State{state: RegisteringNewParticipant, substate: ProcessingParticipantsEmailAddress},
// 	State{state: RegisteringNewParticipant, substate: ProcessingParticipantsEmailAddress}:               State{state: RegisteringNewParticipant, substate: ProcessingParticipantsPassword},
// 	State{state: RegisteringNewParticipant, substate: ProcessingParticipantsPassword, validated: true}:  State{state: AcceptingMessages, substate: NotSet},
// 	State{state: RegisteringNewParticipant, substate: ProcessingParticipantsPassword, validated: false}: State{state: ProcessingMenu, substate: NotSet}, // if the validation fails, transition back to processing the menu

// 	State{state: AuthenticatingParticipant, substate: ProcessingName}:                                   State{state: AuthenticatingParticipant, substate: ProcessingParticipantsPassword},
// 	State{state: AuthenticatingParticipant, substate: ProcessingParticipantsPassword, validated: true}:  State{state: AcceptingMessages, substate: NotSet},
// 	State{state: AuthenticatingParticipant, substate: ProcessingParticipantsPassword, validated: false}: State{state: ProcessingMenu, substate: NotSet}, // if the validation fails, transition back to processing the menu

// 	State{state: CreatingNewChannel, substate: ProcessingName, validated: true}:  State{state: CreatingNewChannel, substate: ProcessingChannelsDesc},
// 	State{state: CreatingNewChannel, substate: ProcessingName, validated: false}: State{state: ProcessingMenu, substate: NotSet},
// 	State{state: CreatingNewChannel, substate: ProcessingChannelsDesc}:           State{state: AcceptingMessages, substate: NotSet},

// 	State{state: SelectingChannel, substate: ProcessingNumber, validated: true}: State{state: }
// }

type Connection struct {
	conn   net.Conn
	ipAddr string

	// TODO(alx): Document in the architecture.md file that a participant only
	// present when the state is Connected.
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
	buffer *bytes.Buffer

	// Placeholders for participant's data.
	participantName           string
	participantEmailAddress   string
	participantPasswordSha256 string

	// Set to true if a participant was auathenticated or registered successfully.
	isConnected bool

	// Current channel selected by the user
	// When a new channel is created, the user is automatically switched to that channel instead
	curChannel   backend.Channel
	channelIsSet bool
}

func newReader(conn net.Conn, connIpAddr string, connectionTimeout time.Duration) *Reader {
	return &Reader{
		conn:              conn,
		ipAddr:            connIpAddr,
		state:             ProcessingMenu,
		connectionTimeout: time.NewTimer(connectionTimeout),
		timeoutDuration:   connectionTimeout,
		idleConnectionsCh: make(chan struct{}),
		abortSignalCh:     make(chan struct{}),
		quitSignalCh:      make(chan struct{}),
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

func (reader *Reader) read() error {
	buffer := make([]byte, 1024)
	bytesRead, err := reader.conn.Read(buffer)
	trimmedBuffer := []byte(strings.Trim(string(buffer[:bytesRead]), " \r\n\v\t\f"))
	reader.buffer = bytes.NewBuffer(trimmedBuffer)

	return err
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
