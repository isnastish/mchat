package main

import (
	"bufio"
	"flag"
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

type Session struct {
	logger              log.Logger
	clients             map[string]*Client
	timer               *time.Timer
	timeout             time.Duration
	nClients            atomic.Int32
	listener            net.Listener
	network             string // tcp|udp
	address             string
	onSessionJoinedCh   chan *Client
	onSessionLeftCh     chan string
	onSessionRejoinedCh chan *Client
	messagesCh          chan Message
}

func NewSession(networkProtocol string, address string) *Session {
	const timeout = 5000 * time.Millisecond

	listener, err := net.Listen(networkProtocol, address)
	if err != nil {
		logger.Error().Msgf("failed to created a listener: %v", err.Error())
		return nil
	}

	return &Session{
		logger:              log.NewLogger("debug"),
		clients:             make(map[string]*Client),
		timeout:             timeout,
		timer:               time.NewTimer(timeout),
		listener:            listener,
		network:             networkProtocol,
		address:             address,
		onSessionJoinedCh:   make(chan *Client),
		onSessionLeftCh:     make(chan string),
		onSessionRejoinedCh: make(chan *Client),
		messagesCh:          make(chan Message),
	}
}

func (s *Session) AcceptConnection() {
	go s.processConnections()

	logger.Info().Msgf("accepting connections: %s\n", s.listener.Addr().String())

	for {
		// Accept will block until we receive a connection,
		// in order to implement a timeout, we will have to wrap it in a go routine.
		conn, err := s.listener.Accept()
		if err != nil {
			logger.Warn().Msgf("client failed to connect: %s", conn.RemoteAddr().String())
			continue
		}

		go handleConnection(conn, s.onSessionJoinedCh, s.onSessionRejoinedCh, s.onSessionLeftCh, s.messagesCh)
	}
}

var logger log.Logger

func (m *Message) Message() string {
	return m.sender + ":" + m.message
}

func handleConnection(conn net.Conn, onSessionJoinedCh, onSessionRejoinedCh chan *Client, onSessionLeftCh chan string, messagesCh chan Message) {
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
		logger.Error().Msgf("error received while scanning the input: %s", err.Error())
	}

	onSessionLeftCh <- connName
	messagesCh <- Message{sender: connName, message: "left"}
	conn.Close()
}

func (s *Session) processConnections() {
	for {
		select {
		case client := <-s.onSessionJoinedCh:
			logger.Info().Msgf("client joined the session: %s", client.name)
			s.clients[client.name] = client
			s.nClients.Add(1)

		case name := <-s.onSessionLeftCh:
			logger.Info().Msgf("client left the session: %s", name)
			s.clients[name].connected = false
			s.nClients.Add(-1)

		case client := <-s.onSessionRejoinedCh:
			logger.Info().Msgf("client rejoined the session: %s", client.name)
			s.clients[client.name].connected = true
			s.clients[client.name].rejoined = true

			// replace the connection, since the privious one was Close()d
			s.clients[client.name].conn = client.conn
			s.nClients.Add(-1)

		case msg := <-s.messagesCh:
			for name, client := range s.clients {
				if strings.Compare(name, msg.sender) != 0 { // Don't send a message back to its owner (find a way how to avoid string comparison)
					logger.Info().Msgf("wrote (%s) to client: %s", msg, name)
					io.WriteString(client.conn, msg.Message())
				}
			}
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
