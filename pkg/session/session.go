package session

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/isnastish/chat/pkg/common"
	bk "github.com/isnastish/chat/pkg/db_backend"
	bk_mysql "github.com/isnastish/chat/pkg/db_backend/mysql"
	bk_redis "github.com/isnastish/chat/pkg/db_backend/redis"
	lgr "github.com/isnastish/chat/pkg/logger"
	sts "github.com/isnastish/chat/pkg/stats"
)

type Session struct {
	clients map[string]*Client
	// mu                  sync.Mutex
	timer               *time.Timer
	duration            time.Duration
	timeoutCh           chan struct{}
	nClients            atomic.Int32
	listener            net.Listener
	network             string // tcp|udp
	address             string
	onSessionJoinedCh   chan *Client // rename to onSessionJoinCh
	onSessionLeftCh     chan string  // rename to onSessionLeave
	onSessionRejoinedCh chan *Client
	messagesCh          chan *Message
	running             bool
	ctx                 context.Context
	cancelCtx           context.CancelFunc
	quitCh              chan struct{}
	db                  map[string]bool // Replace with mysql

	stats     sts.Stats
	dbBackend bk.DatabaseBackend
}

var log = lgr.NewLogger("debug")

func NewSession(networkProtocol, address string) *Session {
	const timeout = 10000 * time.Millisecond

	backend, set := os.LookupEnv("DATABASE_BACKEND")
	if !set {
		backend = bk.BackendType_Redis
	}

	var dbBackend bk.DatabaseBackend
	var err error

	// This should be done when we create a session manager, NOT for each session.
	// If a specified backend fails to initialize, should we terminate the programm, or try other backends?
	switch backend {
	case bk.BackendType_Redis:
		settings := bk_redis.RedisSettings{}
		dbBackend, err = bk_redis.NewRedisBackend(&settings)
		if err != nil {
			log.Error().Msgf("failed to initialize redis backend: %s", err.Error())
			return nil
		}
	case bk.BackendType_Mysql:
		settings := bk_mysql.MysqlSettings{
			Driver:         "mysql",
			DataSrouceName: "root:polechka2003@tcp(127.0.0.1)/test",
		}
		dbBackend, err = bk_mysql.NewMysqlBackend(&settings)
		if err != nil {
			log.Error().Msgf("failed to initialize mysql backend: %s", err.Error())
			return nil
		}
	case bk.BackendType_Memory:
		// not implemented
	default:
		log.Error().Msgf("backend [%s] is not supported", backend)
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	lc := net.ListenConfig{}
	listener, err := lc.Listen(ctx, networkProtocol, address)
	if err != nil {
		log.Error().Msgf("failed to created a listener: %v", err.Error())
		listener.Close()
		_ = cancelCtx
		return nil
	}

	// // test map
	// clientAHash := "@alexey"
	// var clients = map[string]*map[string]string{
	// 	clientAHash:      {"ip_address": "127.0.0.1:9873", "messagesSent": "12", "connectionTime": "22:00:03"},
	// 	"@another_user":  {"ip_address": "127.0.0.1:7777", "messagesSent": "56"},
	// 	"@one_more_user": {"ip_address": "127.0.0.1:9980", "messagesSent": "1230"},
	// }
	// redis.storeMap(clients)
	// m, err := redis.getMap(clientAHash)
	// if err != nil {
	// 	log.Error().Msgf("failed to retrieve map: %s", err.Error())
	// 	listener.Close()
	// 	return nil
	// }
	// for k, v := range m {
	// 	log.Info().Msgf("%s:%s", k, v)
	// }

	return &Session{
		clients:             make(map[string]*Client),
		duration:            timeout,
		timer:               time.NewTimer(timeout),
		listener:            listener,
		network:             networkProtocol,
		address:             address,
		onSessionJoinedCh:   make(chan *Client),
		onSessionLeftCh:     make(chan string),
		onSessionRejoinedCh: make(chan *Client),
		messagesCh:          make(chan *Message),
		ctx:                 ctx,
		cancelCtx:           cancelCtx,
		quitCh:              make(chan struct{}),
		timeoutCh:           make(chan struct{}),
		dbBackend:           dbBackend,
	}
}

func (s *Session) AcceptConnections() {
	go s.processConnections()

	log.Info().Msgf("accepting connections: %s", s.listener.Addr().String())

	go func() {
		<-s.timeoutCh
		close(s.quitCh)

		// Close the listener to force Accept() to return an error.
		s.listener.Close()
		s.cancelCtx()
	}()

Loop:
	for {
		// This will block forewer anyway, we need a more robust way of cancelation.
		// We don't have to wait for clients to end their connections, since we timeout only if
		// no clients connected yet.
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quitCh:
				break Loop
			default:
				log.Warn().Msgf("client failed to connect: %s", conn.RemoteAddr().String())
			}
			continue
		}

		go s.handleConnection(conn)
	}

	sts.DisplayStats(&s.stats, sts.Session)
}

func disconnectIdleClient(conn net.Conn, abortCh, quitCh chan struct{}) {
	const duration = 5 * 60000 * time.Millisecond
	// const duration = 10000 * time.Millisecond
	timeout := time.NewTimer(duration)

	for {
		select {
		// If a program has already received a value from t.C,
		// the timer is known to have expired and the channel drained.
		case <-timeout.C:
			// After closing the connection
			// any blocked Read or Write operations will be unblocked and return errors.
			close(quitCh)
			conn.Close()
		case <-abortCh:
			// stop and drain the channel before resetting the timer
			if !timeout.Stop() {
				<-timeout.C
			}
			// Should only be invoked on expired timers with drained channels
			timeout.Reset(duration)
		}
	}
}

// Introduce a limit on how many attempts client has
// Disconnect the client if it run out of attempts
func (s *Session) readClientName(conn net.Conn, remoteAddr string) bool {
	s.messagesCh <- &Message{
		ownedBySession: true,
		data:           []byte("client_name:"),
		time:           time.Now(),
		hasRecipient:   true,
		recipient:      remoteAddr,
	}

	buf := make([]byte, 4096) // 256
	for {
		bytesRead, err := conn.Read(buf)
		if err != nil && err != io.EOF {
			log.Error().Msgf("failed to read from a connection: %s", conn.RemoteAddr().String())
			return false
		}

		if bytesRead == 0 {
			return false
		}

		expectedClientName := string(common.StripCR(buf, bytesRead))

		if s.dbBackend.ContainsClient(expectedClientName) {
			// send a message that a client already exists.
			// Is it expensive to make a database query every time in a for loop?
			// Maybe a better approach would be to cache all the clients on session's startup,
			// maybe in a map, and then make a lookup in a map itself rather than making a query into the database.
		} else {
			// append client to a database
			s.dbBackend.RegisterClient(expectedClientName, remoteAddr, "connected", time.Now())
		}

		_, exists := s.db[expectedClientName]
		if exists {
			s.messagesCh <- &Message{
				ownedBySession: true,
				data:           []byte("name: [%s] already exists, try a different one"),
				time:           time.Now(),
				hasRecipient:   true,
				recipient:      remoteAddr,
			}
		} else {
			oldStatus := atomic.SwapInt32(&s.clients[remoteAddr].status, ClientStatus_Connected)
			_ = oldStatus // supposed to be Pending

			// persistant storage
			// s.db[k] = true

			return true
		}
	}
}

func (s *Session) handleConnection(conn net.Conn) {
	abortCh := make(chan struct{})
	quitCh := make(chan struct{})

	go disconnectIdleClient(conn, abortCh, quitCh)

	remoteAddr := conn.RemoteAddr().String()

	s.onSessionJoinedCh <- &Client{
		conn:      conn,
		name:      remoteAddr,
		connected: true,
		status:    ClientStatus_Pending,
	}

	if s.readClientName(conn, remoteAddr) {
		if s.nClients.Load() > 1 {
			// Notify the rest of the participants about who joined the session
			s.messagesCh <- &Message{
				ownedBySession: true,
				data:           []byte("Joined"),
				time:           time.Now(),
			}

			s.listParticipants(conn)
		}

		buf := make([]byte, 4096)

		// Use conn.SetDeadline() in order to disconnect idle clients.
		for {
			nBytes, err := conn.Read(buf)
			if err != nil && err != io.EOF {
				// Do determine whether it was an error or it's we who closed the connection.
				// We would have to multiplex.
				select {
				case <-quitCh:
					// Send a message to a client that it has been idle for long period of time,
					// and thus was disconnected from the session.
					log.Info().Msgf("[%15s]: Has been idle for too long, disconnecting", remoteAddr)
				default:
					log.Error().Msgf("failed to read bytes from the client: %s", err.Error())
				}
				break
			}

			if nBytes == 0 {
				break
			}

			// Introduce a better name
			abortCh <- struct{}{}

			// Would it be possible to serialize the message, send it as bytes,
			// and then deserialize it on the client side?
			// so, the message would have a header and a body
			// containing an actual contents.
			s.messagesCh <- MakeMessage(buf[:nBytes], remoteAddr)
		}
	}

	s.onSessionLeftCh <- remoteAddr
	// s.messagesCh <- MakeMessage([]byte("left"), remoteAddr)
	if s.nClients.Load() > 1 {
		s.messagesCh <- &Message{
			ownedBySession: true,
			data:           []byte("Left"),
			time:           time.Now(),
		}
	}

	// Verify what happens if the connection was already closed
	conn.Close()
}

func (s *Session) processConnections() {
	s.running = true

	for s.running {
		select {
		case client := <-s.onSessionJoinedCh:
			log.Info().Msgf("[%15s] joined the session", client.name)

			s.stats.ClientsJoined.Add(1)

			// here supposed to be the name that the client specified
			s.clients[client.name] = client
			s.nClients.Add(1)
			s.timer.Stop()

		case name := <-s.onSessionLeftCh:
			log.Info().Msgf("[%15s] left the session", name)

			s.stats.ClientsLeft.Add(1)

			s.clients[name].connected = false
			s.nClients.Add(-1)

			// Reset the timer if all clients left the session.
			if s.nClients.Load() == 0 {
				s.timer.Reset(s.duration)
			}

		case client := <-s.onSessionRejoinedCh:
			log.Info().Msgf("[%15s] rejoined the session", client.name)

			s.stats.ClientsRejoined.Add(1)

			s.clients[client.name].connected = true
			s.clients[client.name].rejoined = true

			// Overwrite the connection since the previous one was closed
			s.clients[client.name].conn = client.conn
			s.nClients.Add(1)

		case msg := <-s.messagesCh:
			log.Info().Msgf("received message: %s", string(msg.data))

			if !msg.ownedBySession {
				s.stats.MessagesReceived.Add(1)
			}

			if msg.hasRecipient {
				client, exists := s.clients[msg.recipient]
				if exists {
					if client.connected {
						message := FmtMessage(msg)
						messageSize := len(message)

						var bytesWritten int
						for bytesWritten < messageSize {
							n, err := client.conn.Write(message[bytesWritten:])
							if err != nil {
								log.Error().Msgf("failed to send a message to: |%15s|", client.name)
								break
							}
							bytesWritten += n
						}
						if bytesWritten == messageSize {
							s.stats.MessagesSent.Add(1)
						} else {
							s.stats.MessagesDropped.Add(1)
						}
					} else {
						log.Warn().Str("recipient", msg.recipient).Msg("recipient is diconnected")
						s.stats.MessagesDropped.Add(1)
					}
					// else if c.status == ClientStatus_Pending {
					// 	// we queue the messages until we verify that the username is correct
					// }
				} else {
					log.Warn().Str("recipient", msg.recipient).Msg("recipient doesn't exist")
					s.stats.MessagesDropped.Add(1)
				}
			} else {
				s.broadcastMessage(msg)
			}

		case <-s.timer.C:
			log.Info().Msg("session timeout, no clients joined.")

			s.running = false
			close(s.timeoutCh)
		}
	}
}

// Broadcast messages to all connected clients, except the sender
func (s *Session) broadcastMessage(msg *Message) {
	message := FmtMessage(msg)
	messageSize := len(message)

	for name, client := range s.clients {
		if strings.Compare(name, msg.sender) != 0 {
			var bytesWritten int
			for bytesWritten < messageSize {
				n, err := client.conn.Write(message[bytesWritten:])
				if err != nil {
					log.Error().Msgf("failed to send a message to: %s, error: %s", client.name, err.Error())
					break
				}
				bytesWritten += n
			}
			if bytesWritten == messageSize {
				s.stats.MessagesSent.Add(1)
			} else {
				s.stats.MessagesDropped.Add(1)
			}
		}
	}
}

// NOTE: My impression that it would be better to introduce inboxes and outboxes for the session
// as well, so we can better track down send/received messages (in one place),
// instead of calling conn.Write function everywhere in the code and trying to handle if it fails.
// But we would have to define a notion how do distinguish between different message types.
func (s *Session) listParticipants(conn net.Conn) {
	sessionParticipants := "Session's participants: "
	for _, client := range s.clients {
		status := "disconnected"
		if client.connected {
			status, _ = strings.CutPrefix(status, "dis")
		}

		sessionParticipants += fmt.Sprintf("\n\t[%-20s] *%s", client.name, status)
	}
	sender := conn.LocalAddr().String()
	recipient := conn.RemoteAddr().String()

	s.messagesCh <- MakeMessage(
		[]byte(sessionParticipants),
		sender,
		recipient,
	)
}
