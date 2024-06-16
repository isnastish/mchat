// TODO: Improve metrics (probably redesign).
package session

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/isnastish/chat/pkg/backend"
	"github.com/isnastish/chat/pkg/backend/dynamodb"
	"github.com/isnastish/chat/pkg/backend/memory"
	"github.com/isnastish/chat/pkg/backend/redis"
	"github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/types"
)

type Config struct {
	Network string
	Addr    string

	TLSConfig *tls.Config

	SessionTimeout     time.Duration
	ParticipantTimeout time.Duration

	backend.Config
}

type metrics struct {
	joined                  int
	left                    int
	receivedChatMessages    int
	broadcastedChatMessages int
	sentSysMessages         int
}

type session struct {
	config                 *Config
	listener               net.Listener
	connMap                *connectionMap
	shutdownTimer          *time.Timer
	shutdownSignal         chan struct{}
	triggerShutdownProcess chan struct{}
	chatMessages           chan *types.ChatMessage
	sysMessages            chan *types.SysMessage
	storage                backend.Backend
	metrics                metrics
}

func CreateSession(config *Config) *session {
	listener, err := tls.Listen(config.Network, config.Addr, config.TLSConfig)
	if err != nil {
		log.Logger.Panic("Failed to create net.Listener %v", err)
	}

	var storage backend.Backend

	switch config.BackendType {
	case backend.BackendTypeRedis:
		if config.RedisConfig == nil {
			log.Logger.Panic("Redis config is invalid")
		}

		storage, err = redis.NewRedisBackend(config.RedisConfig)
		if err != nil {
			log.Logger.Panic("Redis backend initialization failed %s", err)
		}

	case backend.BackendTypeDynamodb:
		if config.DynamodbConfig == nil {
			log.Logger.Panic("Dynamodb config is invalid")
		}

		storage, err = dynamodb.NewDynamodbBackend(config.DynamodbConfig)
		if err != nil {
			log.Logger.Panic("Dynamodb backend initialization failed %s", err)
		}

	case backend.BackendTypeMemory:
		storage = memory.NewMemoryBackend()
	}

	session := &session{
		connMap:                newConnectionMap(),
		shutdownTimer:          time.NewTimer(config.SessionTimeout * time.Second),
		shutdownSignal:         make(chan struct{}),
		triggerShutdownProcess: make(chan struct{}),
		listener:               listener,
		chatMessages:           make(chan *types.ChatMessage),
		sysMessages:            make(chan *types.SysMessage),
		config:                 config,
		storage:                storage,
	}

	return session
}

func (s *session) Run() {
	go s.processMessages()
	go func() {
		// Block the shutdown process until a signal is received on a triggerShutdownSignal channle.
		<-s.triggerShutdownProcess
		// Used as an identifier to distinguish who shutted down the session.
		s.shutdownSignal <- struct{}{}
		// Causes the Accept() function to produce an error
		// so we can shut down the session gracefully.
		// TODO: Most likely closing the connection should be done before sending shutdownSignal,
		// since the later will block.
		s.listener.Close()
	}()
	log.Logger.Info("Listening: %s", s.listener.Addr().String())

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			// Shut down the session if no paticipants joined for the specified
			// time limit. The shutdownSignal channel is used to distinguish between
			// an ordinary error and our `request` to shut down the session.
			case <-s.shutdownSignal:
				log.Logger.Info("Nobody was able able connect. Shutting down the session")
				return
			default:
				log.Logger.Warn("Connection %s aborted", conn.RemoteAddr().String())
			}
			continue
		}

		log.Logger.Info("Connected: %s", conn.RemoteAddr().String())

		connection := newConn(conn, s.config.ParticipantTimeout*time.Second)
		s.connMap.addConn(connection)
		go s.handleConnection(connection)

		s.shutdownTimer.Stop()
	}
}

func (s *session) sendMsg(msg interface{}) {
	switch msg := msg.(type) {
	case *types.SysMessage:
		s.sysMessages <- msg

	case *types.ChatMessage:
		s.chatMessages <- msg

	default:
		log.Logger.Panic("Invalid message type")
	}
}

func (s *session) handleConnection(conn *connection) {
	reader := newReader(conn)

	if reader._DEBUG_SkipUserdataProcessing {
		reader.displayChatHistory(s)
	}

	for {
		if !reader._DEBUG_SkipUserdataProcessing {
			if matchState(reader.state, stateJoining) || matchState(reader.state, stateProcessingMenu) {
				s.sendMsg(types.BuildSysMsg(optionsStr, reader.conn.ipAddr))
			}
		}

		reader.read(s)

		if !reader.processCommand(s) {

			if !reader._DEBUG_SkipUserdataProcessing {
				// transitionTable[reader.state](reader, s)
				// TODO: Use the transitionTable[reader.state](reader, s) instead of a giant switch statement
				// we only need to make a look up in a table which invokes the corresponding callback.
				switch {
				case matchState(reader.state, stateJoining):
					transitionTable[stateJoining](reader, s)

				case matchState(reader.state, stateProcessingMenu):
					transitionTable[stateProcessingMenu](reader, s)

				case matchState(reader.state, stateRegistration):
					transitionTable[stateRegistration](reader, s)

				case matchState(reader.state, stateAuthentication):
					transitionTable[stateAuthentication](reader, s)

				case matchState(reader.state, stateCreatingChannel):
					transitionTable[stateCreatingChannel](reader, s)

				case matchState(reader.state, stateSelectingChannel):
					transitionTable[stateSelectingChannel](reader, s)

				case matchState(reader.state, stateAcceptingMessages):
					transitionTable[stateAcceptingMessages](reader, s)
				}
			} else {
				reader.updateState(stateAcceptingMessages)
				transitionTable[reader.state](reader, s)
			}
		}

		if matchState(reader.state, stateDisconnecting) {
			transitionTable[reader.state](reader, s)
			break
		}
	}

	if s.connMap.empty() {
		s.shutdownTimer.Reset(s.config.SessionTimeout * time.Second)
	}
}

func (s *session) processMessages() {
	for {
		select {
		case msg := <-s.chatMessages:
			// log.Logger.Info("Broadcasting participant message")
			broadcasted := s.connMap.broadcastMessage(msg)
			s.metrics.broadcastedChatMessages += broadcasted

		case msg := <-s.sysMessages:
			// log.Logger.Info("Broadcasting system message")
			sent := s.connMap.broadcastMessage(msg)
			s.metrics.sentSysMessages = sent

		case <-s.shutdownTimer.C:
			s.triggerShutdownProcess <- struct{}{}
			return
		}
	}
}
