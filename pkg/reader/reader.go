// TODO: All the menu processing has to be done on the client side as well as participant's input
// validation. The reader has to be agnostic, meaning that it should read either from os.Stdin
// or from a remote connection (net.Conn). And the reader should be moved to a seprate package,
// so it can be used in both the Session and the Client packages.
// That way we reduce the amount of states.
// Since the session currently has a lot of side-effects in addition to its purpose (handling connections).
// That way it would be easier to integrate a TLS support as well.
// Input validation should be done on the client side rather than on the session's side,
// except a password hashed with sha256 algorithm.

package reader

import (
	"bytes"
	"net"
	"strings"
	"time"

	"github.com/isnastish/chat/pkg/types"
)

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
	Unset                              ReaderSubstate = 0x0
	ProcessingName                     ReaderSubstate = 0x01
	ProcessingParticipantsPassword     ReaderSubstate = 0x02
	ProcessingParticipantsEmailAddress ReaderSubstate = 0x03
	ProcessingChannelsDesc             ReaderSubstate = 0x04
	ProcessingNumber                   ReaderSubstate = 0x05
)

type Reader struct {
	Conn     net.Conn
	IpAddr   string
	State    ReaderState
	Substate ReaderSubstate

	// // If a participant was idle for too long, we disconnect it
	// // by closing the connection and closing idleConnectionsCh channel
	// // to indicate that the connection was closed intentionally by us.
	// // If a message was sent, we have to send an abort signal,
	// // indicating that a participant is active and there is no need to disconect it.
	// // connection's timeout is reset in that case.
	// // If an error occurs or a client closed the connection while reading bytes from a connection,
	// // a signal sent onto errorSignalCh channel to unblock disconnectIfIdle function.
	// ConnectionTimeout *time.Timer
	// TimeoutDuration   time.Duration

	// // TODO: Better naming
	// IdleConnectionsCh chan struct{}
	// AbortSignalCh     chan struct{}
	// QuitSignalCh      chan struct{}

	// Buffer for storing bytes read from a connection.
	Buffer *bytes.Buffer

	// Placeholders for participant's data.
	ParticipantName           string
	ParticipantEmailAddress   string
	ParticipantPasswordSha256 string

	// Set to true if a participant was auathenticated or registered successfully.
	IsConnected bool

	// Current channel selected by the user
	// When a new channel is created, the user is automatically switched to that channel instead
	CurChannel   *types.Channel
	ChannelIsSet bool
}

func NewReader(conn net.Conn, connIpAddr string, connectionTimeout time.Duration) *Reader {
	return &Reader{
		// conn:              conn,
		// ipAddr:            connIpAddr,
		// state:             ProcessingMenu,
		// connectionTimeout: time.NewTimer(connectionTimeout),
		// timeoutDuration:   connectionTimeout,
		// idleConnectionsCh: make(chan struct{}),
		// abortSignalCh:     make(chan struct{}),
		// quitSignalCh:      make(chan struct{}),
	}
}

func (r *Reader) Read() error {
	buffer := make([]byte, 1024)
	bytesRead, err := r.Conn.Read(buffer)
	trimmedBuffer := []byte(strings.Trim(string(buffer[:bytesRead]), " \r\n\v\t\f"))
	r.Buffer = bytes.NewBuffer(trimmedBuffer)

	return err
}

func (r *Reader) isState(state ReaderState) bool {
	return r.State == state
}

func (r *Reader) isSubstate(substate ReaderSubstate) bool {
	return r.Substate == substate
}

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
