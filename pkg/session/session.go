package session

import (
	_ "crypto/tls"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	lgr "github.com/isnastish/chat/pkg/logger"
	backend "github.com/isnastish/chat/pkg/session/backend"
	dynamodb "github.com/isnastish/chat/pkg/session/backend/dynamodb"
	memory "github.com/isnastish/chat/pkg/session/backend/memory"
	redis "github.com/isnastish/chat/pkg/session/backend/redis"
	"github.com/rs/zerolog/log"
)

type SessionConfig struct {
	Network     string // tcp|udp
	Addr        string
	BackendType backend.BackendType
	Timeout     time.Duration
}

type Session struct {
	connections           *ConnectionMap
	timeoutClock          *time.Timer
	timeoutDuration       time.Duration
	timeoutAbortCh        chan struct{}
	listener              net.Listener
	participantMessagesCh chan *backend.ParticipantMessage
	systemMessagesCh      chan *backend.SystemMessage
	running               bool
	quitCh                chan struct{}
	storage               backend.Backend
	config                SessionConfig

	// metrics
	participantsJoined             atomic.Int32
	participantsLeft               atomic.Int32
	receivedParticipantMessages    atomic.Int32
	broadcastedParticipantMessages atomic.Int32
	droppedParticipantMessages     atomic.Int32
}

var logger = lgr.NewLogger("debug")

func NewSession(config SessionConfig) *Session {
	// TODO(alx): Implement tls security layer.
	listener, err := net.Listen(config.Network, config.Addr)
	if err != nil {
		logger.Error().Msgf("failed to created a listener: %v", err.Error())
		return nil
	}

	session := &Session{
		connections:           newConnectionMap(),
		timeoutClock:          time.NewTimer(config.Timeout),
		timeoutDuration:       config.Timeout,
		timeoutAbortCh:        make(chan struct{}),
		listener:              listener,
		participantMessagesCh: make(chan *backend.ParticipantMessage),
		systemMessagesCh:      make(chan *backend.SystemMessage),
		quitCh:                make(chan struct{}),
		config:                config,
	}

	switch config.BackendType {
	case backend.BackendTypeRedis:
		session.storage = redis.NewBackend()

	case backend.BackendTypeDynamoDB:
		session.storage = dynamodb.NewBackend()

	case backend.BackendTypeMemory:
		session.storage = memory.NewBackend()

	default:
		session.storage = memory.NewBackend()
	}

	return session
}

func (session *Session) Run() {
	go session.processMessages()

	go func() {
		<-session.timeoutAbortCh
		close(session.quitCh)
		session.listener.Close()
	}()

Loop:
	for {
		conn, err := session.listener.Accept()
		if err != nil {
			select {
			case <-session.quitCh:
				// terminate the session if no participants connected
				// for the whole timeout duration.
				log.Info().Msg(
					"no participants connected, shutting down the session",
				)
				break Loop
			default:
				log.Warn().Msgf(
					"client failed to connect: %s",
					conn.RemoteAddr().String(),
				)
			}
			continue
		}
		go session.handleConnection(conn)
	}
}

func (session *Session) handleConnection(conn net.Conn) {
	// stop the session from timing out since at least one participant has joined
	if !session.timeoutClock.Stop() {
		<-session.timeoutClock.C
	}

	// update the metrics
	session.participantsJoined.Add(1)

	connIpAddr := conn.LocalAddr().String()

	receiver := []string{connIpAddr}

	// create a connection
	session.connections.addConn(&Connection{
		conn:        conn,
		ipAddr:      connIpAddr,
		state:       Pending,
		participant: nil,
	})

	// create a new reader(FSM) to handle all the input read from a connection
	reader := newReader(conn, connIpAddr, time.Second*60*24)
	go reader.disconnectIfIdle()

	for !reader.isState(Disconnecting) {
		if reader.isState(ProcessingMenu) {
			menu := buildMenu(session)
			session.systemMessagesCh <- backend.MakeSystemMessage([]byte(menu), []string{connIpAddr})
		}

		input := reader.read()
		if (input.err != nil) && (input.err != io.EOF) {
			select {
			case <-reader.idleConnectionsCh:
				// Notify idle participant that it will be disconnected
				// due to staying idle for too long.
				session.systemMessagesCh <- backend.MakeSystemMessage([]byte("disconnecting..."), receiver)
			default:
				close(reader.quitSignalCh)
			}
			reader.state = Disconnecting
		}

		// NOTE: When an opposite side closes the connection, we read zero bytes.
		// In that case we have to stop disconnectIfIdle function and
		// set the state to Disconnecting.
		if input.bytesRead == 0 {
			close(reader.quitSignalCh)
			reader.state = Disconnecting
		}

		if reader.isState(ProcessingMenu) {
			option, _ := strconv.Atoi(input.asStr)
			switch option {
			case RegisterParticipant:
				reader.state = RegisteringNewParticipant
				reader.substate = ProcessingName

				// send a system message to a participant asking for username
				session.systemMessagesCh <- backend.MakeSystemMessage(usernameMessageContents, receiver)

			case AuthenticateParticipant:
				reader.state = AuthenticatingParticipant
				reader.substate = ProcessingName

				session.systemMessagesCh <- backend.MakeSystemMessage(usernameMessageContents, receiver)

			case CreateChannel:
				if !reader.wasAuthenticated {
					contents := []byte("cannot create a channel, authentication is required")
					session.systemMessagesCh <- backend.MakeSystemMessage(contents, receiver)
				} else {
					reader.state = CreatingNewChannel
					reader.substate = ProcessingName

					// send a message asking to enter channel's name
					session.systemMessagesCh <- backend.MakeSystemMessage(
						channelsNameMessageContents,
						receiver,
					)
				}

			case SelectChannel:
				if !reader.wasAuthenticated {
					contents := []byte("cannot select a channel, authentication is required")
					session.systemMessagesCh <- backend.MakeSystemMessage(contents, receiver)
				} else {
					reader.state = SelectingChannel

					empty, channelList := buildChannelList(session)
					if !empty {
						session.systemMessagesCh <- backend.MakeSystemMessage(
							[]byte(channelList),
							receiver,
						)
					} else {
						session.systemMessagesCh <- backend.MakeSystemMessage(
							[]byte("no channels were created yet"),
							receiver,
						)
					}
				}

			case Exit:
				reader.state = Disconnecting

			default:
				// If participant's input doesn't match any option,
				// sent a message that an option is not supported.
				// State doesn't change.
				contents := []byte(fmt.Sprintf("option {%s} is not supported", input.asStr))
				session.systemMessagesCh <- backend.MakeSystemMessage(contents, receiver)
			}

		} else if reader.isState(RegisteringNewParticipant) {
			if reader.isSubstate(ProcessingName) {
				reader.substate = ProcessingParticipantsEmailAddress
				reader.participantsName = input.asStr

				session.systemMessagesCh <- backend.MakeSystemMessage(emailAddressMessageContents, receiver)

			} else if reader.isSubstate(ProcessingParticipantsEmailAddress) {
				reader.substate = ProcessingParticipantsPassword
				reader.participantsEmailAddress = input.asStr

				session.systemMessagesCh <- backend.MakeSystemMessage(passwordMessageContents, receiver)

			} else if reader.isSubstate(ProcessingParticipantsPassword) {
				reader.participantsPasswordSha256 = Sha256(input.asBytes)
				// TODO(alx): Should we do the input validation at the end,
				// or on each state, after entering the name, after entering the password,
				// and an email address?

				if !validateUsername(reader.participantsName) {
					reader.substate = ProcessingName
				}

				// if !validateEmailAddress(reader.participantsEmailAddress) {
				// 	reader.substate = ProcessingParticipantsEmailAddress
				// }
				// if !validateEmailAddress(reader.participantsPasswordSha256) {
				// 	reader.substate = ProcessingParticipantsPassword
				// }

				if !session.storage.HasParticipant(reader.participantsName) {
					// TODO(alx): Do we need to speciy all the fields when assigning a participan?
					session.connections.assignParticipant(
						connIpAddr,
						&backend.Participant{
							Name:     reader.participantsName,
							JoinTime: time.Now().Format(time.DateTime),
						},
					)

					session.storage.RegisterParticipant(
						reader.participantsName,
						reader.participantsPasswordSha256,
						reader.participantsEmailAddress,
					)
				} else {
					contents := []byte(fmt.Sprintf("participant {%s} already exists", reader.participantsName))
					session.systemMessagesCh <- backend.MakeSystemMessage(contents, receiver)

					reader.state = ProcessingMenu
					reader.substate = NotSet
				}
			}
		} else if reader.isState(AuthenticatingParticipant) {
			if reader.isSubstate(ProcessingName) {
				reader.substate = ProcessingParticipantsPassword
				reader.participantsName = input.asStr

				// send a message to a participant requesting to enter its password
				session.systemMessagesCh <- backend.MakeSystemMessage(
					passwordMessageContents,
					receiver,
				)

			} else if reader.isSubstate(ProcessingParticipantsPassword) {
				reader.participantsPasswordSha256 = Sha256(input.asBytes)

				validationSucceeded := true
				if !validateUsername(reader.participantsName) {
					// TODO(alx): More descriptive error message.
					contents := []byte(fmt.Sprintf("failed to validate the name {%s}", reader.participantsName))
					session.systemMessagesCh <- backend.MakeSystemMessage(contents, receiver)
					validationSucceeded = false
				}

				if !validatePassword(reader.participantsPasswordSha256) {
					contents := []byte(fmt.Sprintf("failed to validate the password {%s}", reader.participantsPasswordSha256))
					session.systemMessagesCh <- backend.MakeSystemMessage(contents, receiver)
					validationSucceeded = false
				}

				if validationSucceeded {
					reader.participantsPasswordSha256 = Sha256(input.asBytes)
					if session.storage.HasParticipant(reader.participantsName) {
						reader.state = AcceptingMessages

						empty, chatHistory := buildChatHistory(session)
						if !empty {
							session.systemMessagesCh <- backend.MakeSystemMessage(
								[]byte(chatHistory),
								receiver,
							)
						}

						empty, participantList := buildParticipantList(session)
						if !empty {
							session.systemMessagesCh <- backend.MakeSystemMessage(
								[]byte(participantList),
								receiver,
							)
						}

					} else {
						contents := []byte("authentication failed, name or password is incorrect")
						session.systemMessagesCh <- backend.MakeSystemMessage(contents, receiver)
					}
				}
			}
		} else if reader.isState(AcceptingMessages) {
			// TODO(alx): Remove the duplications.
			// Maybe accept the slice inside StoreMessage procedure
			// instead of variadic arguments.
			if reader.channelIsSet {
				session.storage.StoreMessage(
					reader.participantsName,
					time.Now().Format(time.DateTime),
					input.asBytes, reader.curChannel.Name,
				)
				session.participantMessagesCh <- backend.MakeParticipantMessage(
					input.asBytes,
					reader.participantsName,
					reader.curChannel.Name,
				)
			} else {
				session.storage.StoreMessage(
					reader.participantsName,
					time.Now().Format(time.DateTime),
					input.asBytes,
				)
				session.participantMessagesCh <- backend.MakeParticipantMessage(
					input.asBytes,
					reader.participantsName,
				)
			}
			session.receivedParticipantMessages.Add(1)

		} else if reader.isState(CreatingNewChannel) {
			// TODO(alx): Channel's name validation.
			// Shouldn't exceed more than 64 characters.
			// Has to contain numbers as well?
			if reader.isSubstate(ProcessingName) {
				reader.curChannel.Name = input.asStr
				reader.substate = ProcessingChannelsDesc

				session.systemMessagesCh <- backend.MakeSystemMessage(
					channelsDescMessageContents,
					receiver,
				)
			} else if reader.isSubstate(ProcessingChannelsDesc) {
				reader.curChannel.Desc = input.asStr
				if session.storage.HasChannel(reader.curChannel.Name) {
					contents := []byte(fmt.Sprintf("channel {%s} aready exists", reader.curChannel.Name))
					session.systemMessagesCh <- backend.MakeSystemMessage(contents, receiver)
					reader.state = ProcessingMenu
				} else {
					session.storage.RegisterChannel(
						reader.curChannel.Name,
						reader.curChannel.Desc,
						reader.participantsName,
					)
					reader.state = AcceptingMessages
				}
			}
		} else if reader.isState(SelectingChannel) {
			// TODO(alx): What if while we were in a process of selecting a channel,
			// another channel has been created or deleted?
			// The problem only arises when the owner has deleted a channel,
			// with creating new onces the index will be incremented and won't affect our selection.
			// Let's ignore the deletion process for now.

			channels := session.storage.GetChannels()
			channelsCount := len(channels)

			index, err := strconv.Atoi(input.asStr)
			if err != nil {
				// Wrong index, print the list of channels once again.
				contents := []byte(fmt.Sprintf("input {%s} doesn't match any channels", input.asStr))
				session.systemMessagesCh <- backend.MakeSystemMessage(contents, receiver)
			} else {
				index = index % channelsCount
				channels := session.storage.GetChannels()
				ch := channels[index]
				reader.curChannel = backend.Channel{
					Name:         ch.Name,
					Desc:         ch.Desc,
					CreationDate: ch.CreationDate,
				}
			}
		}
	}

	// If a participant was able to authenticate, sent a message to all
	// other participants that it was disconnected.
	if reader.wasAuthenticated {
		contents := []byte(fmt.Sprintf("{%s} disconnected", reader.participantsName))
		session.systemMessagesCh <- backend.MakeSystemMessage(contents, []string{})
	}

	// Remove this connection from a connections map regardless whether
	// it succeeded to authenticate or not.
	session.connections.removeConn(connIpAddr)
	conn.Close()

	// since this participant has disconnected, decrement the number of
	// connected participants
	session.participantsLeft.Add(1)

	// reset session's timeout timer if the amount of connected participants went back to zero
	if session.connections.count() == 0 {
		session.timeoutClock.Reset(session.timeoutDuration)
	}
}

func (session *Session) processMessages() {
	session.running = true
	for session.running {
		select {
		case message := <-session.participantMessagesCh:
			dropped, broadcasted := session.connections.broadcastParticipantMessage(message)
			session.broadcastedParticipantMessages.Add(broadcasted)
			session.droppedParticipantMessages.Add(dropped)

		case message := <-session.systemMessagesCh:
			droppedCount, sentCount := session.connections.broadcastSystemMessage(message)
			// NOTE: Not sure whether we need to track down the amount of system messages we sent.
			_ = droppedCount
			_ = sentCount

		case <-session.timeoutClock.C:
			session.running = false
			close(session.timeoutAbortCh)
		}
	}
}
