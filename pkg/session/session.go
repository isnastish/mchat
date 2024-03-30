package session

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"time"

	lgr "github.com/isnastish/chat/pkg/logger"
	sts "github.com/isnastish/chat/pkg/stats"
)

type ClientStatus int

// const (
// 	ClientStatus_Connected ClientStatus = iota
// 	ClientStatus_Disconnected
// 	ClientStatus_Pending // about to be connected
// )

type Client struct {
	conn      net.Conn
	name      string // remoteName
	connected bool
	rejoined  bool
	// status    ClientStatus
}

type Session struct {
	clients             map[string]*Client
	timer               *time.Timer
	duration            time.Duration
	timeoutCh           chan struct{}
	nClients            atomic.Int32
	listener            net.Listener
	network             string // tcp|udp
	address             string
	onSessionJoinedCh   chan *Client
	onSessionLeftCh     chan string
	onSessionRejoinedCh chan *Client
	messagesCh          chan *Message
	running             bool
	ctx                 context.Context
	cancelCtx           context.CancelFunc
	quitCh              chan struct{}

	stats sts.Stats

	// TODO: Replace with mysql
	db map[string]bool
}

var log = lgr.NewLogger("debug")

func NewSession(networkProtocol, address string) *Session {
	const timeout = 10000 * time.Millisecond

	ctx, cancelCtx := context.WithCancel(context.Background())

	lc := net.ListenConfig{}
	listener, err := lc.Listen(ctx, networkProtocol, address)
	if err != nil {
		log.Error().Msgf("failed to created a listener: %v", err.Error())
		listener.Close()
		_ = cancelCtx
		return nil
	}

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

		// should be mysql db
		db: make(map[string]bool),
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

// func (s *Session) promptClientName(conn net.Conn) bool {
// 	// read from connection
// 	buf := make([]byte, 4096) // 256
// 	for {
// 		// We should only read from a connection in one place
// 		bytesRead, err := conn.Read(buf)
// 		if err != nil && err != io.EOF {
// 			log.Error().Msgf("failed to read from a connection: %s", conn.RemoteAddr().String())
// 			return false
// 		}

// 		if bytesRead == 0 {
// 			log.Error().Msg("zero bytes read")
// 			return false
// 		}

// 		// Make a lookup at a database to see whether this name is already occupied.
// 		clientName := string(buf[:bytesRead])

// 		_, exists := s.db[clientName]
// 		if exists {
// 			msg = []byte("already exist, try a different one")
// 			s.messagesCh <- Message{
// 				sender:    conn.LocalAddr().String(),
// 				recipient: conn.RemoteAddr().String(),
// 				data:      msg,
// 				size:      len(msg),
// 			}
// 		} else {
// 			// connection accepted
// 			// change client's status
// 			c := s.clients[conn.RemoteAddr().String()]
// 			c.status = ClientStatus_Connected
// 			c.clientName = clientName

// 			break
// 		}
// 	}
// }

func (s *Session) handleConnection(conn net.Conn) {
	// NOTE: Each client should have its own channel for pushing messages,
	// so we don't overload the main s.messagesCh channel
	// outgoing := make(chan Message)

	// s.promptClientName(conn) // blocks

	abortCh := make(chan struct{})
	quitCh := make(chan struct{})

	go disconnectIdleClient(conn, abortCh, quitCh)

	remoteAddr := conn.RemoteAddr().String()

	s.onSessionJoinedCh <- &Client{
		conn:      conn,
		name:      remoteAddr,
		connected: true,
		// status:    ClientStatus_Pending,
	}

	// sessionAddr := conn.LocalAddr().String()
	// s.messagesCh <- MakeMessage([]byte("client_name:"), senderName, connName)

	// s.messagesCh <- MakeMessage([]byte("joined"), remoteAddr)

	// Make sure it happens after we added a newly arrived participant to a map,
	// because otherwise, the participant won't be found.
	if s.nClients.Load() > 1 {
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
