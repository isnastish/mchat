// TODO: Improve metrics (probably redesign).
// TODO: Pass settings for redis and dynamoDB backends (should be part of Config).
package session

import (
	"net"
	"os"
	"time"

	"github.com/isnastish/chat/pkg/backend"
	"github.com/isnastish/chat/pkg/backend/dynamodb"
	"github.com/isnastish/chat/pkg/backend/memory"
	"github.com/isnastish/chat/pkg/backend/redis"
	"github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/types"
)

type Config struct {
	Network     string
	Addr        string
	BackendType backend.BackendType
	// The timeout should be disable in a production,
	// but in development it simplifies testing.
	SessionTimeout     int64
	ParticipantTimeout int64
}

type metrics struct {
	joined                  int
	left                    int
	receivedChatMessages    int
	broadcastedChatMessages int
	sentSysMessages         int
}

type session struct {
	config                 Config
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

func CreateSession(config Config) *session {
	listener, err := net.Listen(config.Network, config.Addr)
	if err != nil {
		log.Logger.Error("Listener creation failed: %v", err)
		os.Exit(1)
	}

	var storage backend.Backend

	switch config.BackendType {
	case backend.BackendTypeRedis:
		storage, err = redis.NewRedisBackend("127.0.0.1:6379")
		if err != nil {
			log.Logger.Error("Failed to initialize redis backend: %s", err)
			os.Exit(1)
		}

	case backend.BackendTypeDynamodb:
		storage, err = dynamodb.NewDynamodbBackend()
		if err != nil {
			log.Logger.Error("Failed to initialize dynamodb backend: %s", err)
			os.Exit(1)
		}

	case backend.BackendTypeMemory:
		storage = memory.NewMemoryBackend()
	}

	session := &session{
		connMap:                newConnectionMap(),
		shutdownTimer:          time.NewTimer(time.Duration(config.SessionTimeout) * time.Second),
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

		connection := newConn(conn, time.Duration(s.config.ParticipantTimeout)*time.Second)
		s.connMap.addConn(connection)
		go s.handleConnection(connection)

		s.shutdownTimer.Stop()
	}
}

func (s *session) sendMessage(msg interface{}) {
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

	if reader._DEBUG_SkipUsedataProcessing {
		reader.displayChatHistory(s)
	}

	for {
		if !reader._DEBUG_SkipUsedataProcessing {
			if matchState(reader.state, stateJoining) || matchState(reader.state, stateProcessingMenu) {
				s.sendMessage(types.BuildSysMsg(optionsStr, reader.conn.ipAddr))
			}
		}

		reader.read(s)

		if !reader._DEBUG_SkipUsedataProcessing {
			switch {
			case matchState(reader.state, stateJoining):
				reader.onJoiningState(s)

			case matchState(reader.state, stateProcessingMenu):
				reader.onJoiningState(s)

			case matchState(reader.state, stateRegistration):
				reader.onRegisterParticipantState(s)

			case matchState(reader.state, stateAuthentication):
				reader.onAuthParticipantState(s)

			case matchState(reader.state, stateCreatingChannel):
				reader.onCreateChannelState(s)

			case matchState(reader.state, stateSelectingChannel):
				reader.onSelectChannelState(s)

			case matchState(reader.state, stateAcceptingMessages):
				reader.onAcceptMessagesState(s)
			}
		} else {
			reader.updateState(stateAcceptingMessages)
			reader.onAcceptMessagesState(s)
		}

		if matchState(reader.state, stateDisconnecting) {
			reader.onDisconnectState(s)
			break
		}
	}

	if s.connMap.empty() {
		s.shutdownTimer.Reset(time.Duration(s.config.SessionTimeout))
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
