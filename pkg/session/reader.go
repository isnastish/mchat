// TODO: Add an ability to remove participants from the chat.
// TODO: Write an implementation of the bytes buffer so we don't allocate memory on each frame
// but rather reuse the memmory from the previous read by read() procedure.
package session

// TODO: Introduce a set of common commands, it should be very simple.
// For example:
//
// :menu for displaying the menu
// :listm <channel> for listing the members, takes a name of the channel as an optional parameter, in which case it lists all the members in that channel
// :history <period> <channel> for displaying a chat history, it takes a parameter (a period of time for which to display the history and an optional channel name)
// :listc list all channels
// :commands displayes the list of available commands.

import (
	"bytes"
	"io"
	"math/rand"
	"strconv"
	"strings"

	"github.com/isnastish/chat/pkg/commands"
	"github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/types"
	"github.com/isnastish/chat/pkg/utilities"
	"github.com/isnastish/chat/pkg/validation"
)

type readerState int8
type readerSubstate int8
type option int8

const (
	opRegister option = iota
	opAuthenticate
	opCreateChannel
	opSelectChannel
	opListMembers
	opDisplayChat
	opExit
)

const (
	stateNull readerState = iota
	stateJoining
	stateRegistration
	stateAuthentication
	stateAcceptingMessages
	stateCreatingChannel
	stateSelectingChannel
	stateProcessingMenu
	stateDisconnecting

	// This state should be the last
	stateSentinel
)

const (
	substateNull readerSubstate = iota
	substateReadingName
	substateReadingPassword
	substateReadingEmailAddress
	substateReadingChannnelDesc
)

type readerFSM struct {
	conn *connection

	state    readerState
	substate readerSubstate

	buffer *bytes.Buffer

	// Set to true if in development mode.
	// This allows to disable paticipant's data submission process
	// and jump straight to exchaning the messages.
	_DEBUG_SkipUserdataProcessing bool
}

type transitionCallback func(*readerFSM, *session)

var transitionTable map[readerState]transitionCallback

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
	stateTable[stateCreatingChannel] = "StateCreatingChannel"
	stateTable[stateSelectingChannel] = "StateSelectingChannel"
	stateTable[stateProcessingMenu] = "StateProcessingMenu"
	stateTable[stateDisconnecting] = "StateDisconnecting"

	// init transition table
	transitionTable = make(map[readerState]transitionCallback, stateSentinel-stateNull-1)
	transitionTable[stateJoining] = onJoiningState
	transitionTable[stateRegistration] = onRegisterParticipantState
	transitionTable[stateAuthentication] = onAuthParticipantState
	transitionTable[stateAcceptingMessages] = onAcceptMessagesState
	transitionTable[stateCreatingChannel] = onCreateChannelState
	transitionTable[stateSelectingChannel] = onSelectChannelState
	transitionTable[stateProcessingMenu] = onJoiningState // the same callback as for stateJoining
	transitionTable[stateDisconnecting] = onDisconnectState

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
		conn:                          conn,
		substate:                      substateNull,
		state:                         stateJoining,
		_DEBUG_SkipUserdataProcessing: false,
	}
}

func (r *readerFSM) read(session *session) {
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
			session.sendMsg(
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

func (r *readerFSM) processCommand(session *session) bool {
	// Don't process commands while in a joining or processing menu state
	if matchState(r.state, stateJoining) || matchState(r.state, stateProcessingMenu) {
		return false
	}

	result := commands.ParseCommand(r.buffer)
	if result.Error != nil {
		session.sendMsg(types.BuildSysMsg(result.Error.Error(), r.conn.ipAddr))
		return true
	}

	if result.Matched {
		switch result.CommandType {
		case commands.CommandDisplayMenu:
			r.updateState(stateProcessingMenu)

		//  TODO: Do channel name validation
		case commands.CommandDisplayHistory:
			if r.conn.matchState(connectedState) {
				if chathistory := session.storage.GetChatHistory(result.Channel); len(chathistory) > 0 {
					session.sendMsg(types.BuildSysMsg(util.Fmt(buildChatHistory(chathistory)), r.conn.ipAddr))
				} else {
					session.sendMsg(types.BuildSysMsg(util.Fmtln("Empty chat history"), r.conn.ipAddr))
				}
			} else {
				session.sendMsg(types.BuildSysMsg(util.Fmtln("Authentication required"), r.conn.ipAddr))
			}

		case commands.CommandListMembers:
			if r.conn.matchState(connectedState) {
				if members := session.storage.GetParticipants(); len(members) > 0 {
					session.sendMsg(types.BuildSysMsg(buildMembersList(session, members), r.conn.ipAddr))
				} else {
					session.sendMsg(types.BuildSysMsg(util.Fmtln("Empty members list"), r.conn.ipAddr))
				}
			} else {
				session.sendMsg(types.BuildSysMsg(util.Fmtln("Authentication required"), r.conn.ipAddr))
			}

		case commands.CommandListChannels:
			if r.conn.matchState(connectedState) {
				if channels := session.storage.GetChannels(); len(channels) > 0 {
					session.sendMsg(types.BuildSysMsg(buildChannelList(channels), r.conn.ipAddr))
				} else {
					session.sendMsg(types.BuildSysMsg(util.Fmtln("Empty channel list"), r.conn.ipAddr))
				}

			} else {
				session.sendMsg(types.BuildSysMsg(util.Fmtln("Authentication required"), r.conn.ipAddr))
			}

		case commands.CommandListCommands:
			session.sendMsg(types.BuildSysMsg(commands.CommandsBuilder.String(), r.conn.ipAddr))
		}
		return true
	}
	return false
}

func onJoiningState(reader *readerFSM, session *session) {
	if !matchState(reader.state, stateJoining) && !matchState(reader.state, stateProcessingMenu) {
		log.Logger.Panic(
			"Invalid state %s, expected states: %s, %s",
			stateTable[reader.state], stateTable[stateJoining], stateTable[stateProcessingMenu])
	}

	op, err := strconv.Atoi(reader.buffer.String())
	if err != nil {
		session.sendMsg(types.BuildSysMsg(util.Fmtln("{server: %s} Option %s not supported", util.TimeNowStr(), reader.buffer.String()), reader.conn.ipAddr))
	}

	// Since options in the UI start with an index 1
	op -= 1

	switch option(op) {
	case opRegister:
		if matchState(reader.state, stateJoining) {
			session.sendMsg(types.BuildSysMsg(util.Fmt("{server: %s} enter username: ", util.TimeNowStr()), reader.conn.ipAddr))
			reader.updateState(stateRegistration, substateReadingName)
		} else {
			session.sendMsg(types.BuildSysMsg(util.Fmtln("{server: %s} Already registered", util.TimeNowStr()), reader.conn.ipAddr))
		}

	case opAuthenticate:
		if matchState(reader.state, stateJoining) {
			session.sendMsg(types.BuildSysMsg(util.Fmt("{server: %s} enter username: ", util.TimeNowStr()), reader.conn.ipAddr))
			reader.updateState(stateAuthentication, substateReadingName)
		} else {
			session.sendMsg(types.BuildSysMsg(util.Fmtln("{server: %s} Already authenticated", util.TimeNowStr()), reader.conn.ipAddr))
		}

	case opCreateChannel:
		if !matchState(reader.state, stateJoining) {
			session.sendMsg(types.BuildSysMsg(util.Fmt("{server: %s} enter channel name: ", util.TimeNowStr()), reader.conn.ipAddr))
			reader.updateState(stateCreatingChannel, substateReadingName)
		} else {
			session.sendMsg(types.BuildSysMsg(util.Fmtln("{server: %s} Authentication required", util.TimeNowStr()), reader.conn.ipAddr))
		}

	case opSelectChannel:
		// If a participant is already connected and channle's list is not empty,
		// we set the state to be stateSlectingChannel and request to enter channel's id from the user
		if reader.conn.matchState(connectedState) {
			if reader.displayChannels(session) {
				session.sendMsg(types.BuildSysMsg(util.Fmt("{server: %s} enter channel id: ", util.TimeNowStr()), reader.conn.ipAddr))
				reader.updateState(stateSelectingChannel)
			}
		} else {
			session.sendMsg(types.BuildSysMsg(util.Fmtln("{server: %s} Authentication required", util.TimeNowStr()), reader.conn.ipAddr))
		}

	case opListMembers:
		if reader.conn.matchState(connectedState) {
			reader.displayMembers(session)
			reader.updateState(stateAcceptingMessages)
		} else {
			session.sendMsg(types.BuildSysMsg(util.Fmtln("{server: %s} Authentication required", util.TimeNowStr()), reader.conn.ipAddr))
		}

	case opExit:
		reader.updateState(stateDisconnecting)

	default:
		session.sendMsg(types.BuildSysMsg(util.Fmtln("{server: %s} Invalid option %s", util.TimeNowStr(), reader.buffer.String()), reader.conn.ipAddr))
	}
}

func onRegisterParticipantState(reader *readerFSM, session *session) {
	if !matchState(reader.state, stateRegistration) {
		log.Logger.Panic(
			"Invalid state %s, expected %s",
			stateTable[reader.state], stateTable[stateRegistration])
	}

	switch reader.substate {
	case substateReadingName:
		reader.conn.participant.Username = reader.buffer.String()
		session.sendMsg(types.BuildSysMsg(util.Fmt("{server: %s} enter email: ", util.TimeNowStr()), reader.conn.ipAddr))
		reader.updateState(reader.state, substateReadingEmailAddress)

	case substateReadingEmailAddress:
		reader.conn.participant.Email = reader.buffer.String()
		session.sendMsg(types.BuildSysMsg(util.Fmt("{server: %s} enter password: ", util.TimeNowStr()), reader.conn.ipAddr))
		reader.updateState(reader.state, substateReadingPassword)

	case substateReadingPassword:
		reader.conn.participant.Password = reader.buffer.String()
		validate(reader, session)
	}
}

func onAuthParticipantState(reader *readerFSM, session *session) {
	if !matchState(reader.state, stateAuthentication) {
		log.Logger.Panic("Invalid state %s, expected %s", stateTable[reader.state], stateTable[stateAuthentication])
	}

	switch reader.substate {
	case substateReadingName:
		reader.conn.participant.Username = reader.buffer.String()
		session.sendMsg(types.BuildSysMsg(util.Fmt("{server: %s} enter password: ", util.TimeNowStr()), reader.conn.ipAddr))
		// TODO: Return the state/substate as a struct and assign it to reader's state
		// The state itself can be packed into a struct
		// type readerState struct {
		//	 stateType
		//   substate substateType
		// }
		// state := transitionTable[reader.state](reader, session)
		// reader.state = state
		reader.updateState(reader.state, substateReadingName)

	case substateReadingPassword:
		reader.conn.participant.Password = reader.buffer.String()
		validate(reader, session)
	}
}

func onCreateChannelState(reader *readerFSM, session *session) {
	if !matchState(reader.state, stateCreatingChannel) {
		log.Logger.Panic("Invalid state %s, expected %s", stateTable[reader.state], stateTable[stateCreatingChannel])
	}

	switch reader.substate {
	case substateReadingName:
		reader.conn.channel.Name = reader.buffer.String()
		session.sendMsg(types.BuildSysMsg(util.Fmt("{server: %s} enter description: ", util.TimeNowStr()), reader.conn.ipAddr))
		reader.updateState(reader.state, substateReadingChannnelDesc)

	case substateReadingChannnelDesc:
		// TODO: It doesn't really make sense to process channel's description and do the validation of channel's name only after, but let be for now.
		reader.conn.channel.Desc = reader.buffer.String()
		validate(reader, session)
	}
}

func validate(reader *readerFSM, session *session) {
	if !matchState(reader.state, stateRegistration) &&
		!matchState(reader.state, stateAuthentication) &&
		!matchState(reader.state, stateCreatingChannel) {
		log.Logger.Panic(
			"Invalid state %s, expected states %s, %s, %s",
			stateTable[reader.state], stateTable[stateRegistration], stateTable[stateAuthentication], stateTable[stateCreatingChannel])
	}

	if !matchState(reader.state, stateCreatingChannel) {

		if !validation.ValidateName(reader.conn.participant.Username) {
			session.sendMsg(
				types.BuildSysMsg(util.Fmtln("{server: %s} Username %s not valid", util.TimeNowStr(), reader.conn.participant.Username), reader.conn.ipAddr),
			)
			reader.updateState(stateJoining)
			return
		}

		if !validation.ValidatePassword(reader.conn.participant.Password) {
			session.sendMsg(
				types.BuildSysMsg(util.Fmtln("{server: %s} Password {%s} not valid", util.TimeNowStr(), reader.conn.participant.Password), reader.conn.ipAddr),
			)
			reader.updateState(stateJoining)
			return
		}

		if matchState(reader.state, stateRegistration) {
			// TODO: Check whether it's a valid email address by sending a message.
			if !validation.ValidateEmail(reader.conn.participant.Email) {
				session.sendMsg(
					types.BuildSysMsg(util.Fmtln("{server: %s} Email {%s} not valid", util.TimeNowStr(), reader.conn.participant.Email), reader.conn.ipAddr),
				)
				reader.updateState(stateJoining)
				return
			}

			if session.storage.HasParticipant(reader.conn.participant.Username) {
				session.sendMsg(
					types.BuildSysMsg(util.Fmtln("{server: %s} Participant %s already exists", util.TimeNowStr(), reader.conn.participant.Username), reader.conn.ipAddr),
				)
				reader.updateState(stateJoining)
				return
			}

			reader.conn.participant.JoinTime = util.TimeNowStr()

			// TODO: Document this function in the architecture manual
			go reader.conn.disconnectIfIdle()

			// Register the participant in a backend storage
			session.storage.RegisterParticipant(reader.conn.participant)

			// Display chat history to the connected participant
			reader.displayChatHistory(session)

			// TODO: Document this thoroughly in the architecture manual
			session.connMap.markAsConnected(reader.conn.ipAddr)

		} else {
			if !session.storage.AuthParticipant(reader.conn.participant) {
				session.sendMsg(
					types.BuildSysMsg(util.Fmtln("{server: %s} Failed to authenticate participant %s. "+
						"Username or password is incorrect.", util.TimeNowStr(), reader.conn.participant.Username), reader.conn.ipAddr),
				)
				reader.updateState(stateJoining)
				return
			}

			reader.conn.participant.JoinTime = util.TimeNowStr()

			// TODO: Document.
			go reader.conn.disconnectIfIdle()

			// Display chat history to the connected participant
			reader.displayChatHistory(session)

			// TODO: Display chat history
			session.connMap.markAsConnected(reader.conn.ipAddr)
		}

	} else {
		// Channel validation
		if !validation.ValidateName(reader.conn.channel.Name) {
			session.sendMsg(
				types.BuildSysMsg(util.Fmtln("{server: %s} Channel name {%s} is invalid", util.TimeNowStr(), reader.conn.channel.Name), reader.conn.ipAddr),
			)
			reader.updateState(stateProcessingMenu)
			return
		}

		if session.storage.HasChannel(reader.conn.channel.Name) {
			session.sendMsg(
				types.BuildSysMsg(util.Fmtln("{server: %s} Channel {%s} already exist", util.TimeNowStr(), reader.conn.channel.Name), reader.conn.ipAddr),
			)
			reader.updateState(stateProcessingMenu)
			return
		}

		session.storage.RegisterChannel(reader.conn.channel)
	}

	// Set the state to accepting messages if either registration/authentication/channel creation went successfully
	reader.updateState(stateAcceptingMessages)
}

func onSelectChannelState(reader *readerFSM, session *session) {
	if !matchState(reader.state, stateSelectingChannel) {
		log.Logger.Panic("Invalid %s state, expected %s", stateTable[reader.state], stateTable[stateSelectingChannel])
	}

	id, err := strconv.Atoi(reader.buffer.String())
	if err != nil {
		session.sendMsg(types.BuildSysMsg(util.Fmtln("{server: %s} Invalid channel id %s", util.TimeNowStr(), reader.buffer.String()), reader.conn.ipAddr))
		reader.updateState(stateProcessingMenu)
		return
	}

	channels := session.storage.GetChannels()
	if id > 0 && id < len(channels) {
		// Fine as long as nobody uses the information about the channel rather than the reader itself.
		// If we, for example, relied on channel's name in broadcastMessages procedure,
		// the next line would cause race-conditions since we modifying the connection's internal data,
		// which better be done through the connection map.
		reader.conn.channel = channels[id]
		if history := session.storage.GetChatHistory(reader.conn.channel.Name); len(history) > 0 {
			session.sendMsg(types.BuildSysMsg(buildChatHistory(history), reader.conn.ipAddr))
		} else {
			session.sendMsg(types.BuildSysMsg(util.Fmtln("{server: %s} Empty channel history", util.TimeNowStr()), reader.conn.ipAddr))
		}
		reader.updateState(stateAcceptingMessages)
	} else {
		session.sendMsg(types.BuildSysMsg(util.Fmtln("{server: %s} Id %d is out of range", util.TimeNowStr(), id), reader.conn.ipAddr))
		reader.updateState(stateProcessingMenu)
	}
}

func onAcceptMessagesState(reader *readerFSM, session *session) {
	if !matchState(reader.state, stateAcceptingMessages) {
		log.Logger.Panic("Invalid %s state, expected %s", stateTable[reader.state], stateTable[stateAcceptingMessages])
	}

	// The session has received a message from the client, thus the timout process
	// has to be aborted. We send a signal to the abortConnectionTimeout channel which resets.
	// Since the timer will be reset, we cannot close the channel, because we won't be able to reopen it,
	// so we have to send a message instead.
	reader.conn.abortConnectionTimeout <- struct{}{}

	var msg *types.ChatMessage
	if reader._DEBUG_SkipUserdataProcessing {
		index := rand.Intn(len(_DEBUG_FakeParticipantTable) - 1)
		msg = types.BuildChatMsg(reader.buffer.Bytes(), _DEBUG_FakeParticipantTable[index], reader.conn.channel.Name)

	} else {
		// If the channel is an empty string, it won't pass the check inside the backend itself.
		// So it's safe to pass it like this without haveing an if-statement.
		msg = types.BuildChatMsg(reader.buffer.Bytes(), reader.conn.participant.Username, reader.conn.channel.Name)
	}

	// Storage the message in a backend storage.
	session.storage.StoreMessage(msg)

	session.sendMsg(msg)
}

func onDisconnectState(reader *readerFSM, session *session) {
	if !matchState(reader.state, stateDisconnecting) {
		log.Logger.Panic("Invalid %s state, expected %s", stateTable[reader.state], stateTable[stateDisconnecting])
	}

	log.Logger.Info("Disconnected: %s", reader.conn.ipAddr)

	leftParticipantUsername := reader.conn.participant.Username
	// If the context hasn't been canceled yet. Probably the client has chosen to exit the session.
	// We need to invoke cancel() procedure in order for the disconnectIfIdle() goroutine to finish.
	// That prevents us from having go leaks.
	reader.conn.cancel()

	reader.conn.netConn.Close()

	session.connMap.removeConn(reader.conn.ipAddr)

	// Broadcast message to everyone containing a name of who was disconnected
	session.sendMsg(
		types.BuildSysMsg(util.Fmtln("{server: %s} Participant %s disconnected", util.TimeNowStr(), leftParticipantUsername)),
	)
}

func (r *readerFSM) updateState(newState readerState, newSubstate ...readerSubstate) {
	if len(newSubstate) > 0 {
		r.substate = newSubstate[0]
	} else {
		r.substate = substateNull
	}
	r.state = newState
}

func (r *readerFSM) displayChatHistory(session *session) {
	if history := session.storage.GetChatHistory(); len(history) > 0 {
		session.sendMsg(types.BuildSysMsg(buildChatHistory(history), r.conn.ipAddr))
		return
	}
	session.sendMsg(types.BuildSysMsg(util.Fmtln("{server: %s} Empty chat history", util.TimeNowStr()), r.conn.ipAddr))
}

// Returns true if the channel list is non-empty, false otherwise
func (r *readerFSM) displayChannels(session *session) bool {
	if channels := session.storage.GetChannels(); len(channels) > 0 {
		session.sendMsg(types.BuildSysMsg(util.Fmtln(buildChannelList(channels)), r.conn.ipAddr))
		return true
	}

	session.sendMsg(types.BuildSysMsg(util.Fmtln("{server: %s} Empty channel list", util.TimeNowStr()), r.conn.ipAddr))
	return false
}

func (r *readerFSM) displayMembers(session *session) {
	if members := session.storage.GetParticipants(); len(members) > 0 {
		session.sendMsg(types.BuildSysMsg(util.Fmtln(buildMembersList(session, members)), r.conn.ipAddr))
		return
	}
	session.sendMsg(types.BuildSysMsg(util.Fmtln("{server: %s} Empty member list", util.TimeNowStr()), r.conn.ipAddr))
}

func buildChatHistory(history []*types.ChatMessage) string {
	var builder strings.Builder
	for _, msg := range history {
		message := util.Fmtln("{%s:%s} %s", msg.Sender, msg.SentTime, msg.Contents.String())
		builder.WriteString(message)
	}
	return builder.String()
}

func buildChannelList(channels []*types.Channel) string {
	var builder strings.Builder

	builder.WriteString("channels:\n")
	for index, channel := range channels {
		message := util.Fmtln("\t{%d} :%s", index, channel.Name)
		builder.WriteString(message)
	}
	return builder.String()
}

func buildMembersList(session *session, members []*types.Participant) string {
	var builder strings.Builder
	// Iterate over all the participants in a storage,
	// check whether they are in a connection map to verify which status to display
	// `online` or `offline`. If a paticipant is present in a connection map
	// and its status is not Pending, that is online, otherwise offline.
	builder.WriteString("members:\n")
	for _, member := range members {
		if session.connMap.hasConnectedParticipant(member.Username) {
			builder.WriteString(util.Fmtln("\t{%-64s} *%s", member.Username, connStateTable[connectedState]))
			continue
		}
		builder.WriteString(util.Fmtln("\t{%-64s} *%s", member.Username, connStateTable[pendingState]))
	}
	return builder.String()
}
