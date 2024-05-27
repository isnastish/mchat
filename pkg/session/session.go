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
			logger.Info("processing menu")

			option, _ := strconv.Atoi(input.asStr)
			switch option {
			case RegisterParticipant:
				if !reader.isConnected {
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
				logger.Info("exiting session")
				reader.state = Disconnecting

			default:
				logger.Info("unknown option")
				// If participant's input doesn't match any option,
				// sent a message that an option is not supported.
				contents := []byte(fmt.Sprintf("option {%s} is not supported", input.asStr))
				session.systemMessagesCh <- backend.MakeSystemMessage(contents, receiver)
			}

		} else if reader.isState(RegisteringNewParticipant) {
			logger.Info("registering new participant")

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

				validationSucceeded := false
				if validateName(reader.participantsName) {
					if validateEmailAddress(reader.participantsEmailAddress) {
						// Password validation should be done on the raw data rather than on a sha256
						if validatePassword(input.asStr) {
							validationSucceeded = true
						} else {
							session.systemMessagesCh <- backend.MakeSystemMessage(
								[]byte("password validation failed"),
								receiver,
							)
						}
					} else {
						session.systemMessagesCh <- backend.MakeSystemMessage(
							[]byte("email address validation failed"),
							receiver,
						)
					}
				} else {
					session.systemMessagesCh <- backend.MakeSystemMessage(
						[]byte("name validation failed"),
						receiver,
					)
				}

				if validationSucceeded &&
					!session.storage.HasParticipant(reader.participantsName) {
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
					reader.state = ProcessingMenu
					reader.substate = NotSet
					session.systemMessagesCh <- backend.MakeSystemMessage(
						[]byte(fmt.Sprintf("participant {%s} already exists", reader.participantsName)),
						receiver,
					)
				}
			}
		} else if reader.isState(AuthenticatingParticipant) {
			logger.Info("authenticating participant")

			if reader.isSubstate(ProcessingName) {
				reader.substate = ProcessingParticipantsPassword
				reader.participantsName = input.asStr
				session.systemMessagesCh <- backend.MakeSystemMessage(
					passwordMessageContents,
					receiver,
				)

			} else if reader.isSubstate(ProcessingParticipantsPassword) {
				validationSucceeded := true
				if validateName(reader.participantsName) {
					if validatePassword(input.asStr) {
						validationSucceeded = true
					} else {
						session.systemMessagesCh <- backend.MakeSystemMessage(
							[]byte("password validation failed"),
							receiver,
						)
					}
				} else {
					session.systemMessagesCh <- backend.MakeSystemMessage(
						[]byte("name validation failed"),
						receiver,
					)
				}

				reader.participantsPasswordSha256 = Sha256(input.asBytes)
				if validationSucceeded &&
					session.storage.AuthParticipant(reader.participantsName, reader.participantsPasswordSha256) {
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
						[]byte("authentication failed, name or password is incorrect"),
						receiver,
					)
				}
			}
		} else if reader.isState(AcceptingMessages) {
			logger.Info("accepting messages")

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
			logger.Info("creating channel")

			// TODO(alx): What if the channel has already been created?
			// channelIsSet is equal to True?
			if validateName(input.asStr) {
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
			} else {
				reader.state = CreatingNewChannel
				reader.substate = NotSet
				session.systemMessagesCh <- backend.MakeSystemMessage(
					[]byte("channel's name validation failed, try a different name"),
					receiver,
				)
			}
		} else if reader.isState(SelectingChannel) {
			logger.Info("selecting channel")

			// TODO(alx): What if while we were in a process of selecting a channel,
			// another channel has been created or deleted?
			// The problem only arises when the owner has deleted a channel,
			// with creating new onces the index will be incremented and won't affect our selection.
			// Let's ignore the deletion process for now.
			index, err := strconv.Atoi(input.asStr)
			if err != nil {
				_, channelList := buildChannelList(session)
				session.systemMessagesCh <- backend.MakeSystemMessage(
					[]byte(fmt.Sprintf("input {%s} doesn't match any channels", input.asStr)),
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
