package session

import (
	_ "crypto/tls"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
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

type SessionMetrics struct {
	clientsJoined                   int32
	clientsLeft                     int32
	receivedParticipantMessageCount int32
	sentMessageCount                int32
	droppedMessageCount             int32
}

type Session struct {
	connections           *ConnectionMap
	timeoutClock          *time.Timer
	timeoutDuration       time.Duration
	timeoutAbortCh        chan struct{}
	listener              net.Listener
	network               string
	address               string
	participantMessagesCh chan *backend.ParticipantMessage
	systemMessagesCh      chan *backend.SystemMessage
	running               bool
	quitCh                chan struct{}
	metrics               SessionMetrics
	backend               backend.Backend
	config                SessionConfig
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
		session.backend = redis.NewBackend()

	case backend.BackendTypeDynamoDB:
		session.backend = dynamodb.NewBackend()

	case backend.BackendTypeMemory:
		session.backend = memory.NewBackend()

	default:
		session.backend = memory.NewBackend()
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
				// Timeout the session since nobody has connected.
				break Loop
			default:
				log.Warn().Msgf("client failed to connect: %s", conn.RemoteAddr().String())
			}
			continue
		}
		go session.handleConnection(conn)
	}
}

func (session *Session) handleConnection(conn net.Conn) {
	session.timeoutClock.Stop()

	connIpAddr := conn.LocalAddr().String()

	session.connections.addConn(&Connection{
		conn:        conn,
		ipAddr:      connIpAddr,
		state:       Pending,
		participant: nil,
	})

	reader := newReader(conn, connIpAddr, time.Second*60*24)
	go reader.disconnectIfIdle()

	for !reader.isState(Disconnecting) {
		if reader.isState(ProcessingMenu) {
			// Send a message containing menu options.
			contents := []byte(strings.Join(menuOptionsTable, "\n\r"))
			session.systemMessagesCh <- backend.MakeSystemMessage(contents, []string{connIpAddr})
		}

		input := reader.read()
		if (input.err != nil) && (input.err != io.EOF) {
			select {
			case <-reader.idleConnectionsCh:
				// Notify idle participant that it will be disconnected
				// due to staying idle for too long.
				session.systemMessagesCh <- backend.MakeSystemMessage([]byte("disconnecting..."), []string{connIpAddr})
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

			case AuthenticateParticipant:
				reader.state = AuthenticatingParticipant
				reader.substate = ProcessingName

			case CreateChannel:
				if !reader.wasAuthenticated {
					contents := []byte("cannot create a channel, authentication is required")
					session.systemMessagesCh <- backend.MakeSystemMessage(contents, []string{connIpAddr})
				} else {
					reader.state = CreatingNewChannel
					reader.substate = ProcessingName
				}

			case ListChannels:
				if !reader.wasAuthenticated {
					contents := []byte("cannot list channels, authentication is required")
					session.systemMessagesCh <- backend.MakeSystemMessage(contents, []string{connIpAddr})
				} else {
					reader.state = SelectingChannel
				}

			case Exit:
				reader.state = Disconnecting

			default:
				// If participant's input doesn't match any option,
				// sent a message that an option is not supported.
				// State doesn't change.
				contents := []byte(fmt.Sprintf("option {%s} is not supported", input.asStr))
				session.systemMessagesCh <- backend.MakeSystemMessage(contents, []string{connIpAddr})
			}

		} else if reader.isState(RegisteringNewParticipant) {
			if reader.isSubstate(ProcessingName) {
				reader.substate = ProcessingParticipantsEmailAddress
				reader.participantsName = input.asStr
			} else if reader.isSubstate(ProcessingParticipantsEmailAddress) {
				reader.substate = ProcessingParticipantsPassword
				reader.participantsEmailAddress = input.asStr
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

				if !session.backend.HasParticipant(reader.participantsName) {
					// TODO(alx): Do we need to speciy all the fields when assigning a participan?
					session.connections.assignParticipant(
						connIpAddr,
						&backend.Participant{
							Name:     reader.participantsName,
							JoinTime: time.Now().Format(time.DateTime),
						},
					)

					session.backend.RegisterParticipant(
						reader.participantsName,
						reader.participantsPasswordSha256,
						reader.participantsEmailAddress,
					)
				} else {
					contents := []byte(fmt.Sprintf("participant {%s} already exists", reader.participantsName))
					session.systemMessagesCh <- backend.MakeSystemMessage(contents, []string{connIpAddr})

					reader.state = ProcessingMenu
					reader.substate = NotSet
				}
			}
		} else if reader.isState(AuthenticatingParticipant) {
			if reader.isSubstate(ProcessingName) {
				reader.substate = ProcessingParticipantsEmailAddress
				reader.participantsName = input.asStr
			} else if reader.isSubstate(ProcessingParticipantsPassword) {
				reader.participantsPasswordSha256 = Sha256(input.asBytes)

				validationSucceeded := true
				if !validateUsername(reader.participantsName) {
					// TODO(alx): More descriptive error message.
					contents := []byte(fmt.Sprintf("failed to validate the name {%s}", reader.participantsName))
					session.systemMessagesCh <- backend.MakeSystemMessage(contents, []string{connIpAddr})
					validationSucceeded = false
				}

				if !validatePassword(reader.participantsPasswordSha256) {
					contents := []byte(fmt.Sprintf("failed to validate the password {%s}", reader.participantsPasswordSha256))
					session.systemMessagesCh <- backend.MakeSystemMessage(contents, []string{connIpAddr})
					validationSucceeded = false
				}

				if validationSucceeded {
					reader.participantsPasswordSha256 = Sha256(input.asBytes)
					if session.backend.HasParticipant(reader.participantsName) {
						// TODO(alx): If a participant already exists in a connections map,
						// he tried to connect from a different machine.
						// if session.connections.hasParticipant(reader.participantsName) {
						// 	panic("Connection from an unauthorized machine is not premited")
						// }

						reader.state = AcceptingMessages

						chatHistory := make([]string, 0)
						for _, message := range session.backend.GetChatHistory() {
							formated := fmt.Sprintf(
								"[%s:%s] %s",
								message.Sender,
								message.Time,
								message.Contents,
							)
							chatHistory = append(chatHistory, formated)
						}

						session.systemMessagesCh <- backend.MakeSystemMessage(
							[]byte(strings.Join(chatHistory, "\r\n")),
							[]string{connIpAddr},
						)
					} else {
						contents := []byte("authentication failed, name or password is incorrect")
						session.systemMessagesCh <- backend.MakeSystemMessage(contents, []string{connIpAddr})
					}
				}
			}
		} else if reader.isState(AcceptingMessages) {

			session.backend.StoreMessage(
				reader.participantsName,
				time.Now().Format(time.DateTime),
				input.asBytes,
			)
			session.participantMessagesCh <- backend.MakeParticipantMessage(input.asBytes, reader.participantsName)
		} else if reader.isState(CreatingNewChannel) {
			if reader.isSubstate(ProcessingName) {
				reader.channelName = input.asStr
				reader.substate = ProcessingChannelsDesc
			} else if reader.isSubstate(ProcessingChannelsDesc) {
				reader.channelDesc = input.asStr
				// TODO(alx): Channel's name validation
				if session.backend.HasChannel(reader.channelName) {
					contents := []byte(fmt.Sprintf("channel {%s} aready exists", reader.channelName))
					session.systemMessagesCh <- backend.MakeSystemMessage(contents, []string{connIpAddr})
					reader.state = ProcessingMenu
				} else {
					session.backend.RegisterChannel(
						reader.channelName,
						reader.channelDesc,
						reader.participantsName,
					)
					reader.state = AcceptingMessages
				}
			}
		}
	}

	// If a participant was able to authenticate, sent a message to all
	// other participants that it was disconnected.
	if reader.wasAuthenticated {
		contents := []byte(fmt.Sprintf("{%s} has disconnected", reader.participantsName))
		session.systemMessagesCh <- backend.MakeSystemMessage(contents, []string{})
	}

	// Remove this connection from a connections map regardless whether
	// it succeeded to authenticate or not.
	session.connections.removeConn(connIpAddr)
	conn.Close()
}

func (session *Session) processMessages() {
	session.running = true
	for session.running {
		select {
		case message := <-session.participantMessagesCh:
			droppedCount, sentCount := session.connections.broadcastParticipantMessage(message)
			// session.metrics.droppedMessages += droppedCount
			// s.metrics.sentMessagesCount += sentCount
			_ = droppedCount
			_ = sentCount

		case message := <-session.systemMessagesCh:
			droppedCount, sentCount := session.connections.broadcastSystemMessage(message)
			// s.metrics.droppedMessages += droppedCount
			// s.metrics.sentMessagesCount += sentCount
			_ = droppedCount
			_ = sentCount

		case <-session.timeoutClock.C:
			session.running = false
			close(session.timeoutAbortCh)
		}
	}
}
