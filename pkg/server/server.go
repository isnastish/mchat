package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"time"

	log "github.com/isnastish/chat/pkg/logger"
)

type Client struct {
	conn      net.Conn
	name      string
	connected bool
	rejoined  bool // allow clients to rejoin the session
}

type Message struct {
	sender  string
	message string
}

func (m *Message) Message() string {
	return m.sender + ":" + m.message
}

type Session struct {
	logger              log.Logger
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

func NewSession(networkProtocol string, address string) *Session {
	const timeout = 5000 * time.Millisecond

	// Creating context is not necessary since closing the listener will force Accept() function to fail.
	ctx, cancelCtx := context.WithCancel(context.Background())

	lc := net.ListenConfig{}
	listener, err := lc.Listen(ctx, networkProtocol, address)
	if err != nil {
		fmt.Printf("failed to created a listener: %v", err.Error())
		listener.Close()
		return nil
	} else {
		fmt.Println("Successfully created listener")
	}

	return &Session{
		logger:              log.NewLogger("debug"),
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
		quitCh:              make(chan struct{}), // Do we have to alloate memory for the channel?
		timeoutCh:           make(chan struct{}),
	}
}

func (s *Session) AcceptConnection() {
	go s.processConnections()

	s.logger.Info().Msgf("accepting connections: %s\n", s.listener.Addr().String())

	go func() {
		<-s.timeoutCh
		close(s.quitCh)

		// Close the listener to force Accept() to fail.
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
				s.logger.Warn().Msgf("client failed to connect: %s", conn.RemoteAddr().String())
			}
			continue
		}

		go s.handleConnection(conn, s.onSessionJoinedCh, s.onSessionRejoinedCh, s.onSessionLeftCh, s.messagesCh)
	}
}

func (s *Session) handleConnection(conn net.Conn, onSessionJoinedCh, onSessionRejoinedCh chan *Client, onSessionLeftCh chan string, messagesCh chan Message) {
	connName := conn.RemoteAddr().String()
	messagesCh <- Message{sender: connName, message: "joined"}
	onSessionJoinedCh <- &Client{
		conn:      conn,
		name:      connName,
		connected: true,
	}

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		messagesCh <- Message{sender: connName, message: scanner.Text()}
		// // message with username should look like this:
		// // username: <name>
		// if strings.Contains(text, "username:") {
		// 	_, name, ok := strings.Cut(text, " ")
		// 	if ok {
		// 		logger.Debug().Msgf("username: %s", name)
		// 	}
		// 	continue
		// }
		// logger.Debug().Str("Msg", text).Msg("received message")
		// for _, nextConn := range connections {
		// 	if nextConn != newConn {
		// 		if _, err := io.WriteString(nextConn, text); err != nil {
		// 			disconnected <- newConn
		// 			return
		// 		}
		// 	}
		// }
	}

	if err := scanner.Err(); err != nil {
		s.logger.Error().Msgf("error received while scanning the input: %s", err.Error())
	}

	onSessionLeftCh <- connName
	messagesCh <- Message{sender: connName, message: "left"}
	conn.Close()
}

func (s *Session) processConnections() {
	s.running = true

	for s.running {
		select {
		case client := <-s.onSessionJoinedCh:
			s.logger.Info().Msgf("client joined the session: %s", client.name)
			s.clients[client.name] = client
			s.nClients.Add(1)

		case name := <-s.onSessionLeftCh:
			s.logger.Info().Msgf("client left the session: %s", name)
			s.clients[name].connected = false
			s.nClients.Add(-1)

		case client := <-s.onSessionRejoinedCh:
			s.logger.Info().Msgf("client rejoined the session: %s", client.name)
			s.clients[client.name].connected = true
			s.clients[client.name].rejoined = true

			// replace the connection, since the privious one was Close()d
			s.clients[client.name].conn = client.conn
			s.nClients.Add(-1)

		case msg := <-s.messagesCh:
			// TODO: Disconnect idle clients.
			// If we were not receiving any messages for some amount of time, let's say 30 seconds,
			// we have to disconnect that client.
			for name, client := range s.clients {
				if strings.Compare(name, msg.sender) != 0 { // Don't send a message back to its owner (find a way how to avoid string comparison)
					s.logger.Info().Msgf("wrote (%s) to client: %s", msg, name)
					io.WriteString(client.conn, msg.Message())
				}
			}
		case <-s.timer.C:
			s.logger.Info().Msgf("Timeout, no clients joined.")
			s.running = false
			s.timeoutCh <- struct{}{}
		}
	}
}

func main() {
	var networkProtocol string
	var address string

	flag.StringVar(&networkProtocol, "network", "tcp", "network protocol (tcp|udp)")
	flag.StringVar(&address, "address", ":5000", "address to listen in. E.g. localhost:8080")

	session := NewSession(networkProtocol, address)
	session.AcceptConnection()

	// Each client should have an incoming and outgonig channels where he sends/receives messages
	// incomingMessagesCh := make(chan Message)
	// outgoinMessagesCh := make(chan Message)

	// _ = incomingMessagesCh
	// _ = outgoinMessagesCh
}
