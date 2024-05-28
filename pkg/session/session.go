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
		logger.Error("failed to created a listener: %v", err.Error())
		return nil
	}

	if config.Timeout == 0 {
		config.Timeout = 86400 * time.Second // 24h
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

	logger.Info("listening: %s", session.listener.Addr().String())

Loop:
	for {
		conn, err := session.listener.Accept()
		if err != nil {
			select {
			case <-session.quitCh:
				logger.Info("no participants connected, shutting down the session")
				// terminate the session if no participants connected
				// for the whole timeout duration.
				break Loop
			default:
				logger.Warn(
					"client failed to connect: %s",
					conn.RemoteAddr().String(),
				)
			}
			continue
		}

		go session.handleConnection(conn)

		session.timeoutClock.Stop()
	}
}

func (session *Session) handleConnection(conn net.Conn) {
	// update the metrics
	session.participantsJoined.Add(1)

	connIpAddr := conn.RemoteAddr().String()

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

		if err := reader.read(); (err != nil) && (err != io.EOF) {
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
		if reader.buffer.Len() == 0 {
			close(reader.quitSignalCh)
			reader.state = Disconnecting
		}

		if reader.isState(ProcessingMenu) {
			logger.Info("processing menu")

			option, err := strconv.Atoi(reader.buffer.String())
			if err != nil {
				session.systemMessagesCh <- backend.MakeSystemMessage(
					[]byte(fmt.Sprintf("option {%s} is not supported", reader.buffer.String())),
					receiver,
				)
				continue
			}

			switch option {
			case RegisterParticipant:
				if !reader.isConnected {
					logger.Info("registering new participant")

					reader.state = RegisteringNewParticipant
					reader.substate = ProcessingName
					session.systemMessagesCh <- backend.MakeSystemMessage(usernameMessageContents, receiver)
				} else {
					session.systemMessagesCh <- backend.MakeSystemMessage(
						[]byte("already registered"),
						receiver,
					)
				}

			case AuthenticateParticipant:
				if !reader.isConnected {
					logger.Info("authenticating participant")

					reader.state = AuthenticatingParticipant
					reader.substate = ProcessingName
					session.systemMessagesCh <- backend.MakeSystemMessage(usernameMessageContents, receiver)
				} else {
					session.systemMessagesCh <- backend.MakeSystemMessage(
						[]byte("already authenticated"),
						receiver,
					)
				}

			case CreateChannel:
				if !reader.isConnected {
					session.systemMessagesCh <- backend.MakeSystemMessage(
						[]byte("cannot create a channel, authentication is required"),
						receiver,
					)
				} else {
					logger.Info("creating channel")

					reader.state = CreatingNewChannel
					reader.substate = ProcessingName
					session.systemMessagesCh <- backend.MakeSystemMessage(
						channelsNameMessageContents,
						receiver,
					)
				}

			case SelectChannel:
				if !reader.isConnected {
					session.systemMessagesCh <- backend.MakeSystemMessage(
						[]byte("cannot select a channel, authentication is required"),
						receiver,
					)
				} else {
					logger.Info("selecting channel")

					reader.state = SelectingChannel
					empty, channelList := buildChannelList(session)
					if !empty {
						session.systemMessagesCh <- backend.MakeSystemMessage(
							[]byte(channelList),
							receiver,
						)
					} else {
						session.systemMessagesCh <- backend.MakeSystemMessage(
							[]byte(CR("no channels were created yet")),
							receiver,
						)
					}
				}

			case Exit:
				logger.Info("exiting session")
				reader.state = Disconnecting

			default:
				logger.Info("unknown option")
				// If participant's input doesn't match any option,
				// sent a message that an option is not supported.
				contents := []byte(CR(fmt.Sprintf("option {%s} is not supported", reader.buffer.String())))
				session.systemMessagesCh <- backend.MakeSystemMessage(contents, receiver)
			}

		} else if reader.isState(RegisteringNewParticipant) {
			if reader.isSubstate(ProcessingName) {
				reader.substate = ProcessingParticipantsEmailAddress
				reader.participantName = reader.buffer.String()
				session.systemMessagesCh <- backend.MakeSystemMessage(emailAddressMessageContents, receiver)
			} else if reader.isSubstate(ProcessingParticipantsEmailAddress) {
				reader.substate = ProcessingParticipantsPassword
				reader.participantEmailAddress = reader.buffer.String()
				session.systemMessagesCh <- backend.MakeSystemMessage(passwordMessageContents, receiver)
			} else if reader.isSubstate(ProcessingParticipantsPassword) {
				validationSucceeded := false
				if validateName(reader.participantName) {
					if validateEmailAddress(reader.participantEmailAddress) {
						if validatePassword(reader.buffer.String()) {
							validationSucceeded = true
						} else {
							session.systemMessagesCh <- backend.MakeSystemMessage(
								[]byte(CR("password validation failed")),
								receiver,
							)
						}
					} else {
						session.systemMessagesCh <- backend.MakeSystemMessage(
							[]byte(CR("email address validation failed")),
							receiver,
						)
					}
				} else {
					session.systemMessagesCh <- backend.MakeSystemMessage(
						[]byte(CR("name validation failed")),
						receiver,
					)
				}

				if validationSucceeded {
					if !session.storage.HasParticipant(reader.participantName) {
						// NOTE: It's a desired behaviour that we don't specify password and email
						// address, only the name. The comparison while broadcasting messages will
						// be done using a name rather than any other data.
						session.connections.assignParticipant(
							connIpAddr,
							&backend.Participant{
								Name:     reader.participantName,
								JoinTime: time.Now().Format(time.DateTime),
							},
						)

						reader.participantPasswordSha256 = Sha256(reader.buffer.Bytes())
						session.storage.RegisterParticipant(
							reader.participantName,
							reader.participantPasswordSha256,
							reader.participantEmailAddress,
						)

						// Display a chat history
						empty, history := buildChatHistory(session)
						if !empty {
							session.systemMessagesCh <- backend.MakeSystemMessage(
								[]byte(history),
								receiver,
							)
						}

						// Display a participant list
						empty, participantList := buildParticipantList(session)
						if !empty {
							session.systemMessagesCh <- backend.MakeSystemMessage(
								[]byte(participantList),
								receiver,
							)
						}

						// Set the state to accept new messages
						reader.state = AcceptingMessages
						reader.substate = NotSet

					} else {
						reader.state = ProcessingMenu
						reader.substate = NotSet

						session.systemMessagesCh <- backend.MakeSystemMessage(
							[]byte(CR(fmt.Sprintf("participant {%s} already exists", reader.participantName))),
							receiver,
						)
					}
				} else {
					reader.state = ProcessingMenu
					reader.substate = NotSet
				}
			}
		} else if reader.isState(AuthenticatingParticipant) {
			if reader.isSubstate(ProcessingName) {
				reader.substate = ProcessingParticipantsPassword
				reader.participantName = reader.buffer.String()
				session.systemMessagesCh <- backend.MakeSystemMessage(
					passwordMessageContents,
					receiver,
				)
			} else if reader.isSubstate(ProcessingParticipantsPassword) {
				validationSucceeded := true
				if validateName(reader.participantName) {
					if validatePassword(reader.buffer.String()) {
						validationSucceeded = true
					} else {
						session.systemMessagesCh <- backend.MakeSystemMessage(
							[]byte(CR("password validation failed")),
							receiver,
						)
					}
				} else {
					session.systemMessagesCh <- backend.MakeSystemMessage(
						[]byte(CR("name validation failed")),
						receiver,
					)
				}

				if validationSucceeded {
					reader.participantPasswordSha256 = Sha256(reader.buffer.Bytes())
					if session.storage.AuthParticipant(reader.participantName, reader.participantPasswordSha256) {
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
						reader.substate = NotSet
						session.systemMessagesCh <- backend.MakeSystemMessage(
							[]byte(CR("authentication failed, name or password is incorrect")),
							receiver,
						)
					}
				} else {
					reader.state = ProcessingMenu
					reader.substate = NotSet
				}
			}
		} else if reader.isState(AcceptingMessages) {
			if reader.channelIsSet {
				session.storage.StoreMessage(
					reader.participantName,
					time.Now().Format(time.DateTime),
					reader.buffer.Bytes(), reader.curChannel.Name,
				)
				session.participantMessagesCh <- backend.MakeParticipantMessage(
					reader.buffer.Bytes(),
					reader.participantName,
					reader.curChannel.Name,
				)
			} else {
				session.storage.StoreMessage(
					reader.participantName,
					time.Now().Format(time.DateTime),
					reader.buffer.Bytes(),
				)

				// TODO(alx): Figure out a better way of CRLF to a message.
				session.participantMessagesCh <- backend.MakeParticipantMessage(
					[]byte(CR(reader.buffer.String())),
					reader.participantName,
				)
			}

			session.receivedParticipantMessages.Add(1)

		} else if reader.isState(CreatingNewChannel) {
			// TODO(alx): What if the channel has already been created?
			// channelIsSet is equal to True?
			if validateName(reader.buffer.String()) {
				if reader.isSubstate(ProcessingName) {
					reader.curChannel.Name = reader.buffer.String()
					reader.substate = ProcessingChannelsDesc

					session.systemMessagesCh <- backend.MakeSystemMessage(
						channelsDescMessageContents,
						receiver,
					)
				} else if reader.isSubstate(ProcessingChannelsDesc) {
					reader.curChannel.Desc = reader.buffer.String()
					if session.storage.HasChannel(reader.curChannel.Name) {
						contents := []byte(CR(fmt.Sprintf("channel {%s} aready exists", reader.curChannel.Name)))
						session.systemMessagesCh <- backend.MakeSystemMessage(contents, receiver)
						reader.state = ProcessingMenu
					} else {
						session.storage.RegisterChannel(
							reader.curChannel.Name,
							reader.curChannel.Desc,
							reader.participantName,
						)
						reader.state = AcceptingMessages
					}
				}
			} else {
				reader.state = CreatingNewChannel
				reader.substate = NotSet
				session.systemMessagesCh <- backend.MakeSystemMessage(
					[]byte("channel's name validation failed, try a different name"),
					receiver,
				)
			}
		} else if reader.isState(SelectingChannel) {
			// TODO(alx): What if while we were in a process of selecting a channel,
			// another channel has been created or deleted?
			// The problem only arises when the owner has deleted a channel,
			// with creating new onces the index will be incremented and won't affect our selection.
			// Let's ignore the deletion process for now.
			index, err := strconv.Atoi(reader.buffer.String())
			if err != nil {
				_, channelList := buildChannelList(session)
				session.systemMessagesCh <- backend.MakeSystemMessage(
					[]byte(CR(fmt.Sprintf("input {%s} doesn't match any channels", reader.buffer.String()))),
					receiver,
				)
				session.systemMessagesCh <- backend.MakeSystemMessage(
					[]byte(channelList),
					receiver,
				)
			} else {
				channels := session.storage.GetChannels()
				channelsCount := len(channels)
				if index < channelsCount || index >= channelsCount {

				} else {
					ch := channels[index]
					reader.curChannel = backend.Channel{
						Name:         ch.Name,
						Desc:         ch.Desc,
						CreationDate: ch.CreationDate,
					}

					reader.channelIsSet = true

					// Sent a message to the participant containing the chat history in this channel.
					empty, chatHistory := buildChatHistory(session, ch.Name)
					if !empty {
						session.systemMessagesCh <- backend.MakeSystemMessage(
							[]byte(chatHistory),
							receiver,
						)
					}
				}
			}
		}
	}

	// If a participant was able to connect, sent a message to all
	// other participants that it was disconnected.
	if reader.isConnected {
		contents := []byte(CR(fmt.Sprintf("{%s} disconnected", reader.participantName)))
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
			logger.Info("broadcasting participant's message")

			dropped, broadcasted := session.connections.broadcastParticipantMessage(message)
			session.broadcastedParticipantMessages.Add(broadcasted)
			session.droppedParticipantMessages.Add(dropped)

		case message := <-session.systemMessagesCh:
			logger.Info("broadcasting system message")

			droppedCount, sentCount := session.connections.broadcastSystemMessage(message)
			// NOTE: Not sure whether we need to track down the amount of system messages we sent.
			_ = droppedCount
			_ = sentCount

		case <-session.timeoutClock.C:
			logger.Info("session timeout")

			session.running = false
			close(session.timeoutAbortCh)
		}
	}
}
