// TODO: Add an ability to remove participants from the chat.
// TODO: Write an implementation of the bytes buffer so we don't allocate memory on each frame
// but rather reuse the memmory from the previous read by read() procedure.
package session

import (
	"bytes"
	"io"
	"math/rand"
	"strconv"
	"strings"

	"github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/types"
	"github.com/isnastish/chat/pkg/utilities"
	"github.com/isnastish/chat/pkg/validation"
)

type readerState int8
type readerSubstate int8
type option int8

const (
	opRegister      option = 0
	opAuthenticate  option = 0x1
	opCreateChannel option = 0x2
	opSelectChannel option = 0x3
	opListMembers   option = 0x4
	opDisplayChat   option = 0x5
	opExit          option = 0x6
)

const (
	stateNull readerState = 0

	stateJoining           readerState = 0x1
	stateRegistration      readerState = 0x2
	stateAuthentication    readerState = 0x3
	stateAcceptingMessages readerState = 0x4
	stateCreatingChannel   readerState = 0x5
	stateSelectingChannel  readerState = 0x6
	stateProcessingMenu    readerState = 0x7
	stateDisconnecting     readerState = 0x8
)

const (
	substateNull readerSubstate = 0

	substateReadingName         readerSubstate = 0x1
	substateReadingPassword     readerSubstate = 0x2
	substateReadingEmailAddress readerSubstate = 0x3
	substateReadingChannnelDesc readerSubstate = 0x4
	// substate_Reading        readerSubstate = 0x5
)

type readerFSM struct {
	conn *connection

	state    readerState
	substate readerSubstate

	buffer *bytes.Buffer

	// Set to true if in development mode.
	// This allows to disable paticipant's data submission process
	// and jump straight to exchaning the messages.
	_DEBUG_SkipUsedataProcessing bool
}

var optionsStr string
var optionTable []string
var stateTable []string

var _DEBUG_FakeParticipantTable = [10]string{
	"JohnTaylor",
	"LiamMoore",
	"EmmaWilliams",
	"LiamWilson",
	"LiamBrown",
	"EmilySmith",
	"AvaJones",
	"MichaelMoore",
	"JoeSmith",
	"AvaTaylor",
}

func matchState(stateA, stateB readerState) bool {
	return stateA == stateB
}

func matchSubstate(substateA, substateB readerSubstate) bool {
	return substateA == substateB
}

func init() {
	// states table
	stateTable = make([]string, stateDisconnecting-stateNull+1)
	stateTable[stateNull] = "StateNull"
	stateTable[stateJoining] = "StateJoining"
	stateTable[stateRegistration] = "StateRegistration"
	stateTable[stateAuthentication] = "StateAuthentication"
	stateTable[stateAcceptingMessages] = "StateAcceptingMessages"
	stateTable[stateAcceptingMessages] = "StateAcceptingMessages"
	stateTable[stateCreatingChannel] = "StateCreatingChannel"
	stateTable[stateSelectingChannel] = "StateSelectingChannel"
	stateTable[stateProcessingMenu] = "StateProcessingMenu"
	stateTable[stateDisconnecting] = "StateDisconnecting"

	// options table
	optionTable = make([]string, opExit-opRegister+1)
	optionTable[opRegister] = "register"
	optionTable[opAuthenticate] = "log in"
	optionTable[opCreateChannel] = "create channel"
	optionTable[opSelectChannel] = "select channels"
	optionTable[opListMembers] = "list members"
	optionTable[opDisplayChat] = "display chat history"
	optionTable[opExit] = "exit"

	// Build options string so it can be used as a body of a message
	builder := strings.Builder{}
	builder.WriteString("options:\n")

	for i := 0; i < len(optionTable); i++ {
		builder.WriteString(util.Fmt("\t{%d}: %s\n", i+1, optionTable[i]))
	}

	builder.WriteString("\n\tenter option: ")
	optionsStr = builder.String()
}

// TODO: Make it generic an accept io.Reader interface!?
func newReader(conn *connection) *readerFSM {
	return &readerFSM{
		conn:                         conn,
		substate:                     substateNull,
		state:                        stateJoining,
		_DEBUG_SkipUsedataProcessing: false,
	}
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
				types.BuildSysMsg(util.Fmt("You were idle for too long, disconnecting..."), r.conn.ipAddr),
			)

			// Wait 2 seconds before disconnecting the participant.
			util.Sleep(2000)

		default:
			// The cancel will unclock dissconnetIfIdle() procedure so it can finish gracefully
			// without go routine leaks. The case above won't be invoked, since we've already reached
			// the default statement and the state of the reader would be set to disconnectedState.
			// Thus, the next read() never going to happen.
			r.conn.cancel()
		}
		r.updateState(stateDisconnecting)
		return
	}

	// This conditions occurs when an opposite side closed the connection itself.
	// net.Conn.Close() was called.
	if bytesRead == 0 {
		r.updateState(stateDisconnecting)
	}
}

func (r *readerFSM) onJoiningState(chatSession *session) {
	if !matchState(r.state, stateJoining) && !matchState(r.state, stateProcessingMenu) {
		log.Logger.Panic(
			"Invalid state %s, expected states: %s, %s",
			stateTable[r.state], stateTable[stateJoining], stateTable[stateProcessingMenu])
	}

	op, err := strconv.Atoi(r.buffer.String())
	if err != nil {
		chatSession.sendMessage(types.BuildSysMsg(util.Fmtln("Option %s not supported", r.buffer.String()), r.conn.ipAddr))
	}

	// Since options in the UI start with an index 1
	op -= 1

	switch option(op) {
	case opRegister:
		if matchState(r.state, stateJoining) {
			chatSession.sendMessage(types.BuildSysMsg("Username: ", r.conn.ipAddr))
			r.updateState(stateRegistration, substateReadingName)
		} else {
			chatSession.sendMessage(types.BuildSysMsg(util.Fmtln("Already registered"), r.conn.ipAddr))
		}

	case opAuthenticate:
		if matchState(r.state, stateJoining) {
			chatSession.sendMessage(types.BuildSysMsg("Username: ", r.conn.ipAddr))
			r.updateState(stateAuthentication, substateReadingName)
		} else {
			chatSession.sendMessage(types.BuildSysMsg(util.Fmtln("Already authenticated"), r.conn.ipAddr))
		}

	case opCreateChannel:
		if !matchState(r.state, stateJoining) {
			chatSession.sendMessage(types.BuildSysMsg("Channel name: ", r.conn.ipAddr))
			r.updateState(stateCreatingChannel, substateReadingName)
		} else {
			chatSession.sendMessage(types.BuildSysMsg(util.Fmtln("Authentication required"), r.conn.ipAddr))
		}

	case opSelectChannel:
		// If a participant is already connected and channle's list is not empty,
		// we set the state to be stateSlectingChannel and request to enter channel's id from the user
		if r.conn.matchState(connectedState) {
			if r.displayChannels(chatSession) {
				chatSession.sendMessage(types.BuildSysMsg("channel id: ", r.conn.ipAddr))
				r.updateState(stateSelectingChannel)
			}
		} else {
			chatSession.sendMessage(types.BuildSysMsg(util.Fmtln("Authentication required"), r.conn.ipAddr))
		}

	case opListMembers:
		if r.conn.matchState(connectedState) {
			r.displayMembers(chatSession)
			r.updateState(stateAcceptingMessages)
		} else {
			chatSession.sendMessage(types.BuildSysMsg(util.Fmtln("Authentication required"), r.conn.ipAddr))
		}

	case opExit:
		r.updateState(stateDisconnecting)

	default:
		chatSession.sendMessage(types.BuildSysMsg(util.Fmtln("Invalid option %s", r.buffer.String()), r.conn.ipAddr))
	}
}

func (r *readerFSM) onRegisterParticipantState(chatSession *session) {
	if !matchState(r.state, stateRegistration) {
		log.Logger.Panic(
			"Invalid state %s, expected %s",
			stateTable[r.state], stateTable[stateRegistration])
	}

	switch r.substate {
	case substateReadingName:
		r.conn.participant.Username = r.buffer.String()
		chatSession.sendMessage(types.BuildSysMsg("Email: ", r.conn.ipAddr))
		r.updateState(r.state, substateReadingEmailAddress)

	case substateReadingEmailAddress:
		r.conn.participant.Email = r.buffer.String()
		chatSession.sendMessage(types.BuildSysMsg("Password: ", r.conn.ipAddr))
		r.updateState(r.state, substateReadingPassword)

	case substateReadingPassword:
		r.conn.participant.Password = r.buffer.String()
		r.validate(chatSession)
	}
}

func (r *readerFSM) onAuthParticipantState(chatSession *session) {
	if !matchState(r.state, stateAuthentication) {
		log.Logger.Panic("Invalid state %s, expected %s", stateTable[r.state], stateTable[stateAuthentication])
	}

	switch r.substate {
	case substateReadingName:
		r.conn.participant.Username = r.buffer.String()
		chatSession.sendMessage(types.BuildSysMsg("Password: ", r.conn.ipAddr))
		r.updateState(r.state, substateReadingName)

	case substateReadingPassword:
		r.conn.participant.Password = r.buffer.String()
		r.validate(chatSession)
	}
}

func (r *readerFSM) onCreateChannelState(chatSession *session) {
	if !matchState(r.state, stateCreatingChannel) {
		log.Logger.Panic("Invalid state %s, expected %s", stateTable[r.state], stateTable[stateCreatingChannel])
	}

	switch r.substate {
	case substateReadingName:
		r.conn.channel.Name = r.buffer.String()
		chatSession.sendMessage(types.BuildSysMsg("Description: ", r.conn.ipAddr))
		r.updateState(r.state, substateReadingChannnelDesc)

	case substateReadingChannnelDesc:
		// TODO: It doesn't really make sense to process channel's description and do the validation of channel's name only after, but let be for now.
		r.conn.channel.Desc = r.buffer.String()
		r.validate(chatSession)
	}
}

func (r *readerFSM) validate(chatSession *session) {
	if !matchState(r.state, stateRegistration) &&
		!matchState(r.state, stateAuthentication) &&
		!matchState(r.state, stateCreatingChannel) {
		log.Logger.Panic(
			"Invalid state %s, expected states %s, %s, %s",
			stateTable[r.state], stateTable[stateRegistration], stateTable[stateAuthentication], stateTable[stateCreatingChannel])
	}

	if !matchState(r.state, stateCreatingChannel) {

		if !validation.ValidateName(r.conn.participant.Username) {
			chatSession.sendMessage(
				types.BuildSysMsg(util.Fmtln("Username %s not valid", r.conn.participant.Username), r.conn.ipAddr),
			)
			r.updateState(stateJoining)
			return
		}

		if !validation.ValidatePassword(r.conn.participant.Password) {
			chatSession.sendMessage(
				types.BuildSysMsg(util.Fmtln("Password {%s} not valid", r.conn.participant.Password), r.conn.ipAddr),
			)
			r.updateState(stateJoining)
			return
		}

		if matchState(r.state, stateRegistration) {
			// TODO: Check whether it's a valid email address by sending a message.
			if !validation.ValidateEmail(r.conn.participant.Email) {
				chatSession.sendMessage(
					types.BuildSysMsg(util.Fmtln("Email {%s} not valid", r.conn.participant.Email), r.conn.ipAddr),
				)
				r.updateState(stateJoining)
				return
			}

			if chatSession.storage.HasParticipant(r.conn.participant.Username) {
				chatSession.sendMessage(
					types.BuildSysMsg(util.Fmtln("Participant %s already exists", r.conn.participant.Username), r.conn.ipAddr),
				)
				r.updateState(stateJoining)
				return
			}

			r.conn.participant.JoinTime = util.TimeNowStr()

			// TODO: Document this function in the architecture manual
			go r.conn.disconnectIfIdle()

			// Register the participant in a backend storage
			chatSession.storage.RegisterParticipant(r.conn.participant)

			// Display chat history to the connected participant
			r.displayChatHistory(chatSession)

			// TODO: Document this thoroughly in the architecture manual
			chatSession.connMap.markAsConnected(r.conn.ipAddr)

		} else {
			if !chatSession.storage.AuthParticipant(r.conn.participant) {
				chatSession.sendMessage(
					types.BuildSysMsg(util.Fmtln("Failed to authenticate participant %s. "+
						"Username or password is incorrect.", r.conn.participant.Username), r.conn.ipAddr),
				)
				r.updateState(stateJoining)
				return
			}

			r.conn.participant.JoinTime = util.TimeNowStr()

			// TODO: Document.
			go r.conn.disconnectIfIdle()

			// Display chat history to the connected participant
			r.displayChatHistory(chatSession)

			// TODO: Display chat history
			chatSession.connMap.markAsConnected(r.conn.ipAddr)
		}

	} else {
		// Channel validation
		if !validation.ValidateName(r.conn.channel.Name) {
			chatSession.sendMessage(
				types.BuildSysMsg(util.Fmtln("Channel name {%s} is invalid", r.conn.channel.Name), r.conn.ipAddr),
			)
			r.updateState(stateProcessingMenu)
			return
		}

		if chatSession.storage.HasChannel(r.conn.channel.Name) {
			chatSession.sendMessage(
				types.BuildSysMsg(util.Fmtln("Channel {%s} already exist", r.conn.channel.Name), r.conn.ipAddr),
			)
			r.updateState(stateProcessingMenu)
			return
		}

		chatSession.storage.RegisterChannel(r.conn.channel)
	}

	// Set the state to accepting messages if either registration/authentication/channel creation went successfully
	r.updateState(stateAcceptingMessages)
}

func (r *readerFSM) onSelectChannelState(chatSession *session) {
	if !matchState(r.state, stateSelectingChannel) {
		log.Logger.Panic("Invalid %s state, expected %s", stateTable[r.state], stateTable[stateSelectingChannel])
	}

	id, err := strconv.Atoi(r.buffer.String())
	if err != nil {
		chatSession.sendMessage(types.BuildSysMsg(util.Fmtln("Invalid channel id %s", r.buffer.String()), r.conn.ipAddr))
		r.updateState(stateProcessingMenu)
		return
	}

	channels := chatSession.storage.GetChannels()
	if id > 0 && id < len(channels) {
		// Fine as long as nobody uses the information about the channel rather than the reader itself.
		// If we, for example, relied on channel's name in broadcastMessages procedure,
		// the next line would cause race-conditions since we modifying the connection's internal data,
		// which better be done through the connection map.
		r.conn.channel = channels[id]
		if history := chatSession.storage.GetChannelHistory(r.conn.channel.Name); len(history) > 0 {
			chatSession.sendMessage(types.BuildSysMsg(buildChatHistory(history), r.conn.ipAddr))
		} else {
			chatSession.sendMessage(types.BuildSysMsg(util.Fmtln("Empty channel history"), r.conn.ipAddr))
		}
		r.updateState(stateAcceptingMessages)
	} else {
		chatSession.sendMessage(types.BuildSysMsg(util.Fmt("Id %d is out of range", id), r.conn.ipAddr))
		r.updateState(stateProcessingMenu)
	}
}

func (r *readerFSM) onAcceptMessagesState(chatSession *session) {
	if !matchState(r.state, stateAcceptingMessages) {
		log.Logger.Panic("Invalid %s state, expected %s", stateTable[r.state], stateTable[stateAcceptingMessages])
	}

	// The session has received a message from the client, thus the timout process
	// has to be aborted. We send a signal to the abortConnectionTimeout channel which resets.
	// Since the timer will be reset, we cannot close the channel, because we won't be able to reopen it,
	// so we have to send a message instead.
	r.conn.abortConnectionTimeout <- struct{}{}

	var msg *types.ChatMessage
	if r._DEBUG_SkipUsedataProcessing {
		index := rand.Intn(len(_DEBUG_FakeParticipantTable) - 1)
		msg = types.BuildChatMsg(r.buffer.Bytes(), _DEBUG_FakeParticipantTable[index], r.conn.channel.Name)

	} else {
		// If the channel is an empty string, it won't pass the check inside the backend itself.
		// So it's safe to pass it like this without haveing an if-statement.
		msg = types.BuildChatMsg(r.buffer.Bytes(), r.conn.participant.Username, r.conn.channel.Name)
	}

	// Storage the message in a backend storage.
	chatSession.storage.StoreMessage(msg)

	chatSession.sendMessage(msg)
}

func (r *readerFSM) onDisconnectState(chatSession *session) {
	if !matchState(r.state, stateDisconnecting) {
		log.Logger.Panic("Invalid %s state, expected %s", stateTable[r.state], stateTable[stateDisconnecting])
	}

	if r.conn.participant.Username != "" {
		chatSession.sendMessage(
			types.BuildSysMsg(util.Fmtln("Participant %s disconnected", r.conn.participant.Username)),
		)
	}

	// If the context hasn't been canceled yet. Probably the client has chosen to exit the session.
	// We need to invoke cancel() procedure in order for the disconnectIfIdle() goroutine to finish.
	// That prevents us from having go leaks.
	r.conn.cancel()

	r.conn.netConn.Close()
	chatSession.connMap.removeConn(r.conn.ipAddr)
}

func (r *readerFSM) updateState(newState readerState, newSubstate ...readerSubstate) {
	if len(newSubstate) > 0 {
		r.substate = newSubstate[0]
	} else {
		r.substate = substateNull
	}
	r.state = newState
}

func (r *readerFSM) displayChatHistory(chatSession *session) {
	if history := chatSession.storage.GetChatHistory(); len(history) > 0 {
		chatSession.sendMessage(types.BuildSysMsg(buildChatHistory(history), r.conn.ipAddr))
		return
	}
	chatSession.sendMessage(types.BuildSysMsg(util.Fmtln("Empty chat history"), r.conn.ipAddr))
}

// Returns true if the channel list is non-empty, false otherwise
func (r *readerFSM) displayChannels(chatSession *session) bool {
	if channels := chatSession.storage.GetChannels(); len(channels) > 0 {
		chatSession.sendMessage(types.BuildSysMsg(util.Fmtln(buildChannelList(channels)), r.conn.ipAddr))
		return true
	}

	chatSession.sendMessage(types.BuildSysMsg(util.Fmtln("Empty channel list"), r.conn.ipAddr))
	return false
}

func (r *readerFSM) displayMembers(chatSession *session) {
	if members := chatSession.storage.GetParticipants(); len(members) > 0 {
		chatSession.sendMessage(types.BuildSysMsg(util.Fmtln(buildMembersList(chatSession, members)), r.conn.ipAddr))
		return
	}
	chatSession.sendMessage(types.BuildSysMsg(util.Fmtln("Empty member list"), r.conn.ipAddr))
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
