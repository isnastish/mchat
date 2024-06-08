// TODO: Add an ability to remove participants from the chat.
// TODO: Write an implementation of the bytes buffer so we don't allocate memory on each frame
// but rather reuse the memmory from the previous read by read() procedure.
package session

import (
	"bytes"
	"io"
	"strconv"
	"strings"

	"github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/types"
	"github.com/isnastish/chat/pkg/utilities"
	"github.com/isnastish/chat/pkg/validation"
)

type readerState int8
type readerSubstate int8

const (
	registerParticipant = 0
	authParticipant     = 0x1
	createChannel       = 0x2
	selectChannel       = 0x3
	listMembers         = 0x4
	displayChatHistory  = 0x5
	exit                = 0x6
)

var optionTable []string
var optionsStr string

const (
	nullState readerState = 0

	joiningState             readerState = 0x1
	registerParticipantState readerState = 0x2
	authParticipantState     readerState = 0x3
	acceptMessagesState      readerState = 0x4
	createChannelState       readerState = 0x5
	selectChannelState       readerState = 0x6
	disconnectState          readerState = 0x7
	processMenuState         readerState = 0x8
	validateState            readerState = 0x9
)

const (
	null readerSubstate = 0

	readName      readerSubstate = 0x1
	readPassword  readerSubstate = 0x2
	readEmail     readerSubstate = 0x3
	readDesc      readerSubstate = 0x4
	readChanIndex readerSubstate = 0x5
)

type readerFSM struct {
	conn *connection

	prevState readerState
	state     readerState
	substate  readerSubstate

	buffer *bytes.Buffer
}

func initOptions() {
	// Initialize options table and options string only once.
	if len(optionTable) == 0 {
		optionTable = make([]string, exit-registerParticipant+1)

		optionTable[registerParticipant] = "register"
		optionTable[authParticipant] = "log in"
		optionTable[createChannel] = "create channel"
		optionTable[selectChannel] = "select channels"
		optionTable[listMembers] = "list members"
		optionTable[displayChatHistory] = "display chat history"
		optionTable[exit] = "exit"

		builder := strings.Builder{}
		builder.WriteString("options:\n")

		for i := 0; i < len(optionTable); i++ {
			builder.WriteString(util.Fmt("\t{%d}: %s\n", i+1, optionTable[i]))
		}

		builder.WriteString("\n\tenter option: ")

		optionsStr = builder.String()
	}
}

// TODO: Make it generic an accept io.Reader interface!?
func newReader(conn *connection) *readerFSM {
	initOptions()
	return &readerFSM{
		conn:  conn,
		state: joiningState,
	}
}

func matchState(stateA, stateB readerState) bool {
	return stateA == stateB
}

func matchSubstate(substateA, substateB readerSubstate) bool {
	return substateA == substateB
}

func (r *readerFSM) read(chatSession *session) {
	// net.Conn.Read() method accepts the bytes of non-zero length,
	// thus creating a bytes.Buffer{} and passing it as buffer.Bytes() wouldn't work.
	// The subsequent operation on the buffer.Len() would return 1024 instead of an actual amount of bytes read from a connection.
	buffer := make([]byte, 1024)
	bytesRead, err := r.conn.netConn.Read(buffer)
	trimmedBuffer := util.TrimWhitespaces(buffer[:bytesRead])
	r.buffer = bytes.NewBuffer(trimmedBuffer)

	if err != nil && err != io.EOF {
		select {
		case <-r.conn.ctx.Done():
			// Send a message to the client notifying that he has been idle for too long.
			// The timeout duration is set by the session.
			// The client gets disconnected.
			chatSession.sendMessage(
				types.BuildSysMsg("You were idle for too long, disconnecting...", r.conn.ipAddr),
			)
		default:
			// The cancel will unclock dissconnetIfIdle() procedure so it can finish gracefully
			// without go routine leaks. The case above won't be invoked, since we've already reached
			// the default statement and the state of the reader would be set to disconnectedState.
			// Thus, the next read() never going to happen.
			r.conn.cancel()
		}

		r.state = disconnectState
		return
	}

	// This conditions occurs when an opposite side closed the connection itself.
	// net.Conn.Close() was called.
	if bytesRead == 0 {
		r.state = disconnectState
	}
}

func (r *readerFSM) onJoiningState(chatSession *session) {
	if !matchState(r.state, joiningState) && !matchState(r.state, processMenuState) {
		log.Logger.Panic("Invalid state")
	}

	option, err := strconv.Atoi(r.buffer.String())
	if err != nil {
		chatSession.sendMessage(types.BuildSysMsg(util.Fmt("Option %s not supported", r.buffer.String()), r.conn.ipAddr))
	}

	// Since options in the UI start with an index 1
	option -= 1

	switch option {
	case registerParticipant:
		if matchState(r.state, joiningState) {
			chatSession.sendMessage(types.BuildSysMsg("Username: ", r.conn.ipAddr))
			r.prevState = joiningState
			r.substate = readName
			r.state = registerParticipantState
			return
		}
		chatSession.sendMessage(types.BuildSysMsg("Already registered", r.conn.ipAddr))

	case authParticipant:
		if matchState(r.state, joiningState) {
			chatSession.sendMessage(types.BuildSysMsg("Username: ", r.conn.ipAddr))
			r.prevState = joiningState
			r.substate = readName
			r.state = authParticipantState
			return
		}
		chatSession.sendMessage(types.BuildSysMsg("Already authenticated", r.conn.ipAddr))

	case createChannel:
		if !matchState(r.state, joiningState) {
			chatSession.sendMessage(types.BuildSysMsg("Channel name: ", r.conn.ipAddr))
			r.prevState = joiningState
			r.substate = readName
			r.state = createChannelState
			return
		}
		chatSession.sendMessage(types.BuildSysMsg("Authentication required", r.conn.ipAddr))

	case selectChannel:
		if !matchState(r.state, joiningState) {
			if channels := chatSession.storage.GetChannels(); len(channels) > 0 {
				chatSession.sendMessage(types.BuildSysMsg(buildChannelList(channels), r.conn.ipAddr))
				chatSession.sendMessage(types.BuildSysMsg("Channel's index: ", r.conn.ipAddr))
				r.prevState = joiningState
				r.state = selectChannelState
			} else {
				chatSession.sendMessage(types.BuildSysMsg("No channels were created", r.conn.ipAddr))
			}
			return
		}
		chatSession.sendMessage(types.BuildSysMsg("Authentication required", r.conn.ipAddr))

	case listMembers:
		if !matchState(r.state, joiningState) {
			if members := chatSession.storage.GetParticipants(); len(members) > 0 {
				chatSession.sendMessage(types.BuildSysMsg(buildMembersList(chatSession, members), r.conn.ipAddr))
			}
			r.prevState = r.state
			r.state = acceptMessagesState
			return
		}
		chatSession.sendMessage(types.BuildSysMsg("Authentication required", r.conn.ipAddr))

	case exit:
		r.state = disconnectState

	default:
		// Not setting the prevState so that then menu can be displayed.
		chatSession.sendMessage(types.BuildSysMsg("Invalid option", r.conn.ipAddr))
	}
}

func (r *readerFSM) onRegisterParticipantState(chatSession *session) {
	if !matchState(r.state, registerParticipantState) {
		log.Logger.Panic("Invalid state")
	}

	switch {
	case matchSubstate(r.substate, readName):
		r.conn.participant.Username = r.buffer.String()
		r.substate = readEmail
		chatSession.sendMessage(types.BuildSysMsg("Email: ", r.conn.ipAddr))

	case matchSubstate(r.substate, readEmail):
		r.conn.participant.Email = r.buffer.String()
		r.substate = readPassword
		chatSession.sendMessage(types.BuildSysMsg("Password: ", r.conn.ipAddr))

	case matchSubstate(r.substate, readPassword):
		r.conn.participant.Password = r.buffer.String()
		r.substate = null
		r.prevState = r.state

		r.validate(chatSession)
	}
}

func (r *readerFSM) onAuthParticipantState(chatSession *session) {
	if !matchState(r.state, authParticipantState) {
		log.Logger.Panic("Invalid state")
	}

	switch {
	case matchSubstate(r.substate, readName):
		r.conn.participant.Username = r.buffer.String()
		r.substate = readPassword
		chatSession.sendMessage(types.BuildSysMsg("Password: ", r.conn.ipAddr))

	case matchSubstate(r.substate, readPassword):
		r.conn.participant.Password = r.buffer.String()
		r.substate = null
		r.prevState = r.state
		r.state = validateState
	}
}

func (r *readerFSM) onCreateChannelState(chatSession *session) {
	if !matchState(r.state, createChannelState) {
		log.Logger.Panic("Invalid state")
	}

	switch {
	case matchSubstate(r.substate, readName):
		r.conn.channel.Name = r.buffer.String()
		r.substate = readDesc
		chatSession.sendMessage(types.BuildSysMsg("Description: ", r.conn.ipAddr))

	case matchSubstate(r.substate, readDesc):
		r.conn.channel.Desc = r.buffer.String()
		r.substate = null
		r.prevState = r.state
		r.state = acceptMessagesState
	}
}

func (r *readerFSM) validate(chatSession *session) {
	// NOTE: How do we decide which data to validate?
	if !matchState(r.state, registerParticipantState) &&
		!matchState(r.state, authParticipantState) &&
		!matchState(r.state, createChannelState) {
		log.Logger.Panic("Invalid state")
	}

	if !matchState(r.state, createChannelState) {

		if !validation.ValidateName(r.conn.participant.Username) {
			chatSession.sendMessage(
				types.BuildSysMsg(util.Fmt("Username {%s} not valid", r.conn.participant.Username), r.conn.ipAddr),
			)
			return
		}

		if !validation.ValidatePassword(r.conn.participant.Password) {
			chatSession.sendMessage(
				types.BuildSysMsg(util.Fmt("Password {%s} not valid", r.conn.participant.Password), r.conn.ipAddr),
			)
			return
		}

		if matchState(r.state, registerParticipantState) {
			// TODO: Check whether it's a valid email address by sending a message.
			if !validation.ValidateEmail(r.conn.participant.Email) {
				chatSession.sendMessage(
					types.BuildSysMsg(util.Fmt(util.EndOfLine("Email {%s} not valid"), r.conn.participant.Email), r.conn.ipAddr),
				)
			}

			if chatSession.storage.HasParticipant(r.conn.participant.Username) {
				chatSession.sendMessage(
					types.BuildSysMsg(util.Fmt(util.EndOfLine("Participant {%s} already exists"), r.conn.participant.Username), r.conn.ipAddr),
				)
				r.substate = null
				r.prevState = nullState
				r.state = joiningState
				return
			}

			r.conn.participant.JoinTime = util.TimeNowStr()

			// TODO: Document this function in the architecture manual
			go r.conn.disconnectIfIdle()

			chatSession.storage.RegisterParticipant(r.conn.participant)

			// Display chat history to the connected participant
			if chatHistory := chatSession.storage.GetChatHistory(); len(chatHistory) > 0 {
				chatSession.sendMessage(types.BuildSysMsg(buildChatHistory(chatHistory), r.conn.ipAddr))
			}

			// TODO: Document this thoroughly in the architecture manual.
			chatSession.connMap.markAsConnected(r.conn.ipAddr)

		} else {
			if !chatSession.storage.AuthParticipant(r.conn.participant) {
				chatSession.sendMessage(
					types.BuildSysMsg(util.Fmt("Failed to authenticate participant {%s}. "+
						"Username or password is incorrect.", r.conn.participant.Username), r.conn.ipAddr),
				)
				r.substate = null
				r.prevState = nullState
				r.state = joiningState
				return
			}

			r.conn.participant.JoinTime = util.TimeNowStr()

			// TODO: Document.
			go r.conn.disconnectIfIdle()

			// Display chat history to the connected participant
			if chatHistory := chatSession.storage.GetChatHistory(); len(chatHistory) > 0 {
				chatSession.sendMessage(types.BuildSysMsg(buildChatHistory(chatHistory), r.conn.ipAddr))
			}

			// TODO: Display chat history
			chatSession.connMap.markAsConnected(r.conn.ipAddr)
		}

	} else {
		// Channel validation
		if !validation.ValidateName(r.conn.channel.Name) {
			chatSession.sendMessage(
				types.BuildSysMsg(util.Fmt("Channel name {%s} is invalid", r.conn.channel.Name), r.conn.ipAddr),
			)
			return
		}
	}

	r.substate = null
	r.prevState = r.state
	r.state = acceptMessagesState
}

func (r *readerFSM) onSelectChannelState(chatSession *session) {
	if !matchState(r.state, selectChannelState) {
		log.Logger.Panic("Invalid state")
	}

	// TODO: Give a participant three attempts to select a channel,
	// if he fails all of them, get back to displaying the menu. (Probably)
	channelID, err := strconv.Atoi(r.buffer.String())
	if err != nil {
		chatSession.sendMessage(types.BuildSysMsg("Index {%s} is invalid", r.conn.ipAddr))
		r.substate = null
		r.state = processMenuState
		return
	}

	channels := chatSession.storage.GetChannels()
	if channelID > 0 && channelID < len(channels) {
		// Fine as long as nobody uses the information about the channel rather than the reader itself.
		// If we, for example, relied on channel's name in broadcastMessages procedure,
		// the next line would cause race-conditions.
		r.conn.channel = channels[channelID]

		if history := chatSession.storage.GetChannelHistory(r.conn.channel.Name); len(history) > 0 {
			chatSession.sendMessage(types.BuildSysMsg(buildChatHistory(history), r.conn.ipAddr))
		}

		r.prevState = r.state
		r.substate = null
		r.state = acceptMessagesState

		return
	}

	chatSession.sendMessage(
		types.BuildSysMsg(util.Fmt("ChannelID {%d} is out of range", channelID), r.conn.ipAddr),
	)

	r.substate = null
	r.state = processMenuState
}

func (r *readerFSM) onAcceptMessagesState(chatSession *session) {
	if !matchState(r.state, acceptMessagesState) {
		log.Logger.Panic("Invalid state")
	}

	// The session has received a message from the client, thus the timout process
	// has to be aborted. We send a signal to the abortConnectionTimeout channel which resets.
	// Since the timer will be reset, we cannot close the channel, because we won't be able to reopen it,
	// so we have to send a message instead.
	r.conn.abortConnectionTimeout <- struct{}{}

	var msg *types.ChatMessage

	if r.conn.channel.Name != "" {
		msg = types.BuildChatMsg(r.buffer.Bytes(), r.conn.participant.Username, r.conn.channel.Name)
	} else {
		msg = types.BuildChatMsg(r.buffer.Bytes(), r.conn.participant.Username)
	}

	chatSession.storage.StoreMessage(msg)
	chatSession.sendMessage(msg)
}

func (r *readerFSM) onDisconnectState(chatSession *session) {
	if !matchState(r.state, disconnectState) {
		log.Logger.Panic("Invalid state")
	}

	if r.conn.participant.Username != "" {
		chatSession.sendMessage(
			types.BuildSysMsg(util.Fmt("Participant {%s} disconnected", r.conn.participant.Username)),
		)
	}

	// If the context hasn't been canceled yet. Probably the client has chosen to exit the session.
	// We need to invoke cancel() procedure in order for the disconnectIfIdle() goroutine to finish.
	// That prevents us from having go leaks.
	r.conn.cancel()

	r.conn.netConn.Close()
	chatSession.connMap.removeConn(r.conn.ipAddr)
}

func buildChatHistory(history []*types.ChatMessage) string {
	var builder strings.Builder
	for _, msg := range history {
		message := util.Fmt("{%s:%s} %s", msg.Sender, msg.SentTime, msg.Contents.String())
		builder.WriteString(message)
	}
	return builder.String()
}

func buildChannelList(channels []*types.Channel) string {
	var builder strings.Builder

	builder.WriteString("channels:\n")
	for index, channel := range channels {
		message := util.Fmt("\t{%d} :%s\n", index, channel.Name)
		builder.WriteString(message)
	}
	return builder.String()
}

func buildMembersList(chatSession *session, members []*types.Participant) string {
	var builder strings.Builder
	// Iterate over all the participants in a storage,
	// check whether they are in a connection map to verify which status to display
	// `online` or `offline`. If a paticipant is present in a connection map
	// and its status is not Pending, that is online, otherwise offline.
	builder.WriteString("members:\n")
	for _, member := range members {
		if chatSession.connMap.hasConnectedParticipant(member.Username) {
			builder.WriteString(util.Fmt("\t{%-64s} *%s\n", member.Username, connStateTable[connectedState]))
			continue
		}
		builder.WriteString(util.Fmt("\t{%-64s} *%s\n", member.Username, connStateTable[pendingState]))
	}
	return builder.String()
}
