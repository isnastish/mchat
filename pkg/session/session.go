package session

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"time"

	l "github.com/isnastish/chat/pkg/logger"
)

type Client struct {
	conn      net.Conn
	name      string
	connected bool
	rejoined  bool
}

type Message struct {
	sender string
	data   []byte
}

type Session struct {
	// log                 l.Logger
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
	messagesCh          chan Message
	running             bool
	ctx                 context.Context
	cancelCtx           context.CancelFunc
	quitCh              chan struct{}
}

var log = l.NewLogger("debug")

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
	} else {
		log.Info().Msg("Successfully created listener")
	}

	return &Session{
		// log:                 log,
		clients:             make(map[string]*Client),
		duration:            timeout,
		timer:               time.NewTimer(timeout),
		listener:            listener,
		network:             networkProtocol,
		address:             address,
		onSessionJoinedCh:   make(chan *Client),
		onSessionLeftCh:     make(chan string),
		onSessionRejoinedCh: make(chan *Client),
		messagesCh:          make(chan Message),
		ctx:                 ctx,
		cancelCtx:           cancelCtx,
		quitCh:              make(chan struct{}),
		timeoutCh:           make(chan struct{}),
	}
}

func (s *Session) AcceptConnection() {
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
}

func disconnectIfClientWasIdle(conn net.Conn, abortCh, quitCh chan struct{}) {
	const duration = 10000 * time.Millisecond // 10s (for test)
	timeout := time.NewTimer(duration)

	for {
		select {
		case <-timeout.C:
			// Close closes the connection.
			// Any blocked Read or Write operations will be unblocked and return errors.
			{
				close(quitCh)
				conn.Close()
			}
		case <-abortCh:
			log.Info().Msg("timeout aborted")
			{
				if !timeout.Stop() {
					<-timeout.C // drain the channel (might block)
				}
				log.Info().Msg("channel was drained")
				// Should only be invoked on expired timers with drained channels
				timeout.Reset(duration)
			}
		}
	}
}

func (s *Session) handleConnection(conn net.Conn) {
	// NOTE: Each client should have its own channel for pushing messages,
	// so we don't overload the main s.messagesCh channel
	// outgoing := make(chan Message)

	abortCh := make(chan struct{})
	quitCh := make(chan struct{})

	go disconnectIfClientWasIdle(conn, abortCh, quitCh)

	connName := conn.RemoteAddr().String()
	conn.Write([]byte(fmt.Sprintf("your username: %s", connName)))

	if s.nClients.Load() != 0 {
		s.listParticipants(conn)
	}

	s.messagesCh <- Message{sender: connName, data: []byte("joined")}
	s.onSessionJoinedCh <- &Client{
		conn:      conn,
		name:      connName,
		connected: true,
	}

	buf := make([]byte, 4096)

	for {
		nBytes, err := conn.Read(buf)
		if err != nil && err != io.EOF {
			// Do determine whether it was an error or it's we who closed the connection.
			// We would have to multiplex.
			select {
			case <-quitCh:
				// Send a message to a client that it has been idle for long period of time,
				// and thus was disconnected from the session.
				log.Error().Msgf("client has been idle for too long, disconnecting")
			default:
				log.Error().Msgf("failed to read bytes from the client: %s", err.Error())
			}
			break
		}

		// // figure out when do we read zero bytes.
		// if nBytes == 0 {
		// 	break
		// }

		// Introduce a better name
		abortCh <- struct{}{}

		// Would it be possible to serialize the message, send it as bytes,
		// and then deserialize it on the client side?
		// so, the message would have a header and a body
		// containing an actual contents.

		log.Info().Int("count", nBytes).Msg("read bytes")
		s.messagesCh <- Message{
			sender: connName,
			data:   buf[:nBytes],
		}
	}

	log.Info().Msg("sending connection leave message")
	s.onSessionLeftCh <- connName
	log.Info().Msg("message about client leaving was sent")
	s.messagesCh <- Message{sender: connName, data: []byte("left")}

	// Verify what happens if the connection was already closed
	conn.Close()
}

func (s *Session) processConnections() {
	s.running = true

	for s.running {
		select {
		case client := <-s.onSessionJoinedCh:
			log.Info().Msgf("client joined the session: %s", client.name)
			{
				s.clients[client.name] = client
				s.nClients.Add(1)

				if s.timer.Stop() {
					log.Info().Msg("the timer was stopped from firing")
				}
			}

		case name := <-s.onSessionLeftCh:
			log.Info().Msgf("client left the session: %s", name)
			{
				s.clients[name].connected = false
				s.nClients.Add(-1)

				// Reset the timer if all clients left the session.
				if s.nClients.Load() == 0 {
					s.timer.Reset(s.duration)
				}
			}

		case client := <-s.onSessionRejoinedCh:
			log.Info().Msgf("client rejoined the session: %s", client.name)
			{
				s.clients[client.name].connected = true
				s.clients[client.name].rejoined = true

				// replace the connection, since the privious one was Close()d
				s.clients[client.name].conn = client.conn
				s.nClients.Add(1)
			}

		case msg := <-s.messagesCh:
			log.Info().Msgf("[%s]: received message: %s", msg.sender, string(msg.data))
			{
				msgStr := fmt.Sprintf("[%s]: %s", msg.sender, string(msg.data))
				msgBytes := []byte(msgStr)

				// TODO: Disconnect idle clients.
				// If we were not receiving any messages for some amount of time, let's say 30 seconds,
				// we have to disconnect that client.
				for name, client := range s.clients {
					if strings.Compare(name, msg.sender) != 0 { // Don't send a message back to its owner (find a way how to avoid string comparison)
						log.Info().Msgf("wrote (%s) to client: %s", msg, name)
						nBytes, err := client.conn.Write(msgBytes)
						if err != nil {
							log.Error().Msgf("failed to write to client: %s", name)
							continue
						}

						// If we couldn't write everything at once. This part is not tested!
						for nBytes < len(msgBytes) {
							n, err := client.conn.Write(msgBytes[nBytes:])
							if err != nil {
								log.Error().Msgf("failed to write to client: %s", name)
								break
							}
							nBytes += n
						}
					}
				}
			}

		case <-s.timer.C:
			log.Info().Msgf("timeout, no clients joined.")
			{
				s.running = false
				close(s.timeoutCh)
			}
		}
	}
}

func (s *Session) listParticipants(conn net.Conn) {
	p := make([]string, len(s.clients))
	for _, client := range s.clients {
		// We can even display all the participants,
		// and display whether they are connected or disconnected.
		if client.connected {
			p = append(p, client.name)
		}
	}

	_, err := conn.Write([]byte(fmt.Sprintf("\n\nParticipants:\n%s", strings.Join(p, "\n"))))
	if err != nil {
		log.Error().Msgf("failed to write participants list: %s", err.Error())
	}
}
