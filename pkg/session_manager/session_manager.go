package session

import (
	"net"
	"sync"

	bk "github.com/isnastish/chat/pkg/backend"
	bk_memory "github.com/isnastish/chat/pkg/backend/memory"
	bk_mysql "github.com/isnastish/chat/pkg/backend/mysql"
	bk_redis "github.com/isnastish/chat/pkg/backend/redis"
	lgr "github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/session"
)

type Settings struct { // unused for now
	NetworkProtocol string
	Addr            string
	BackendType     int
}

// When a session manager starts, we should fetch all the sessions from a storage into a local map?
// But this is an optimization, and should be done at the end.

type SessionManager struct {
	// a session's storage
	backend bk.SessionStorageBackend

	// listener to handle all incomming connections
	l net.Listener

	// address to listen in
	addr string

	// network protocol tcp|udp
	protocol string

	// port to listen in (should be a part of address for simplisity)
	port int

	// Let's start with creating only one session first, test it thoroughly,
	// and then add an ability for clients to create their own sessions.
	// sessions map[string]*Session
	session *session.Session

	// wait for all the sessions to finish and then shutdown the server
	wg sync.WaitGroup
}

var log = lgr.NewLogger("debug")

func (sm *SessionManager) InitSessionManager(networkProtocol string, addr string, backendType int) bool {
	var backend bk.SessionStorageBackend
	var err error

	switch backendType {
	case bk.BackendType_Redis:
		backend, err = bk_redis.NewRedisBackend(&bk_redis.RedisSettings{
			Network:    "tcp",
			Addr:       "127.0.0.1:6379",
			Password:   "",
			MaxRetries: 5,
		})

		if err != nil {
			log.Error().Msgf("failed to initialize redis backend: %s", err.Error())
			return false
		}

		log.Info().Msgf("using backend: %s", bk.BackendTypeStr(backendType))

	case bk.BackendType_Mysql:
		backend, err = bk_mysql.NewMysqlBackend(&bk_mysql.MysqlSettings{
			Driver:         "mysql",
			DataSrouceName: "root:polechka2003@tcp(127.0.0.1)/test",
		})

		if err != nil {
			log.Error().Msgf("failed to initialize mysql backend: %s", err.Error())
			return false
		}

		log.Info().Msgf("using backend: %s", bk.BackendTypeStr(backendType))

	case bk.BackendType_Memory:
		backend = bk_memory.NewMemoryBackend()

		log.Info().Msgf("using backend: %s", bk.BackendTypeStr(backendType))

	default:
		log.Warn().Msgf("[%s] is not supported", bk.BackendTypeStr(backendType))
		backend = bk_memory.NewMemoryBackend()

		log.Info().Msgf("using backend: %s", bk.BackendTypeStr(bk.BackendType_Memory))
	}

	listener, err := net.Listen(networkProtocol, addr)
	if err != nil {
		log.Error().Msgf("failed to created a listener: %v", err.Error())
		listener.Close()
		return false
	}

	sm.backend = backend
	sm.addr = addr
	sm.protocol = networkProtocol
	sm.l = listener
	sm.wg = sync.WaitGroup{}

	return true
}

func (sm *SessionManager) ProcessConnections() {
	// Load all the sessions from a storage and spin them up?
	// Then distribute incomming clients by sessions.
	// A client should choose the session that he wishes to join,
	// from a list of available sessions.
	// Each session should be executed in a separate go routine.
	// When client decides to specify a session on his own,
	// the session should be added to a list of all sessions.
	// When nobody joined the session for some period of time,
	// we should terminate it.
	// So each session should have its own timeout value.

	// log.Info().Msgf("accepting connections: %s", s.listener.Addr().String())

	// go func() {
	// 	<-s.timeoutCh
	// 	close(s.quitCh)

	// 	// Close the listener to force Accept() to return an error.
	// 	s.listener.Close()
	// 	s.cancelCtx()
	// }()

Loop:
	for {
		// This will block forewer anyway, we need a more robust way of cancelation.
		// We don't have to wait for clients to end their connections, since we timeout only if
		// no clients connected yet.
		client, err := sm.l.Accept()
		if err != nil {
			select {
			case <-s.quitCh: // no active sessions left.
				break Loop
			default:
				log.Warn().Msgf("client failed to connect: %s", client.RemoteAddr().String())
			}
			continue
		}

		go sm.handleClient(client)
	}
}

func (sm *SessionManager) beginSession(name string) {
	// onSessionBegin
}

func (sm *SessionManager) endSession(name string) bool {
	// onSessionClose

	return true
}
