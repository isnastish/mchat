package session

// A session itself should hold all the messages send by different clients, not the client itself,
// because the question arises what kind of messages each client has to store? Only the ones that he sent?

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	bk "github.com/isnastish/chat/pkg/backend"
	bk_memory "github.com/isnastish/chat/pkg/backend/memory"
	bk_mysql "github.com/isnastish/chat/pkg/backend/mysql"
	bk_redis "github.com/isnastish/chat/pkg/backend/redis"
	"github.com/isnastish/chat/pkg/common"
	lgr "github.com/isnastish/chat/pkg/logger"
	sts "github.com/isnastish/chat/pkg/stats"
)

// Most likely we won't need to pull settings into a struct,
// if we end up having only three member fields.
type Settings struct {
	NetworkProtocol string // tcp|udp
	Addr            string
	BackendType     int
}

type ChatHistory struct {
	_messages []ClientMessage
	_count    int32
	_mu       sync.Mutex
}

func (h *ChatHistory) push(msg *ClientMessage) {
	h._mu.Lock()
	h._messages = append(h._messages, *msg)
	h._count++
	h._mu.Unlock()
}

func (h *ChatHistory) size() int32 {
	h._mu.Lock()
	defer h._mu.Unlock()
	return h._count
}

func (h *ChatHistory) flush(out []ClientMessage) int32 {
	h._mu.Lock()
	n := copy(out, h._messages)
	h._mu.Unlock()
	return int32(n)
}

type ClientMap struct {
	_clientsList []*Client // used for getClients()
	_count       int32
	_clientsMap  map[string]*Client
	_mu          sync.Mutex
}

func NewClientMap() *ClientMap {
	return &ClientMap{
		_clientsMap: make(map[string]*Client),
	}
}

func (m *ClientMap) add(addr string, c *Client) {
	m._mu.Lock()
	m._clientsMap[addr] = c
	m._clientsList = append(m._clientsList, c)
	m._count++
	m._mu.Unlock()
}

func (m *ClientMap) updateStatus(addr string, status int32) {
	m._mu.Lock()
	m._clientsMap[addr].status = status
	m._mu.Unlock()
}

func (m *ClientMap) assignName(addr string, name string) {
	m._mu.Lock()
	m._clientsMap[addr].name = name
	m._mu.Unlock()
}

func (m *ClientMap) size() int32 {
	m._mu.Lock()
	defer m._mu.Unlock()
	return m._count
}

func (m *ClientMap) getClients(out []*Client) {
	m._mu.Lock()
	copy(out, m._clientsList)
	m._mu.Unlock()
}

func (m *ClientMap) tryGet(addr string, out *Client) bool {
	m._mu.Lock()
	defer m._mu.Unlock()
	c, exists := m._clientsMap[addr]
	if exists {
		*out = *c
	}
	return exists
}

// This function will be removed when we add a database support.
func (m *ClientMap) existsWithName(addr string, name string) bool {
	m._mu.Lock()
	defer m._mu.Unlock()
	c, exists := m._clientsMap[addr]
	if exists && strings.Compare(c.name, name) == 0 {
		return true
	}
	return false
}

type Session struct {
	name               string
	clients            *ClientMap
	timer              *time.Timer
	duration           time.Duration
	timeoutCh          chan struct{}
	nClients           atomic.Int32
	listener           net.Listener
	network            string // tcp|udp
	address            string
	onSessionJoinCh    chan *Client
	onSessionLeaveCh   chan string
	clientMessagesCh   chan *ClientMessage
	sessionMessagesCh  chan *SessionMessage
	greetingMessagesCh chan *GreetingMessage
	running            bool
	quitCh             chan struct{}
	stats              sts.Stats // atomic
	backend            bk.DatabaseBackend
	chatHistory        ChatHistory

	// When a session starts, we should upload all the client names from our backend database
	// into a runtimeDB, so we can operate on a map itself, rather than making queries to a database
	// when we need to verify user's name.
	// runtimeDB map[string]bool
}

var log = lgr.NewLogger("debug")

func NewSession(settings *Settings) *Session {
	const timeout = 10000 * time.Millisecond

	var dbBackend bk.DatabaseBackend
	var err error

	switch settings.BackendType {
	case bk.BackendType_Redis:
		dbBackend, err = bk_redis.NewRedisBackend(&bk_redis.RedisSettings{
			Network:    "tcp",
			Addr:       "127.0.0.1:6379",
			Password:   "",
			MaxRetries: 5,
		})

		if err != nil {
			log.Error().Msgf("failed to initialize redis backend: %s", err.Error())
			return nil
		}

		log.Info().Msgf("using backend: %s", bk.BackendTypeStr(settings.BackendType))

	case bk.BackendType_Mysql:
		dbBackend, err = bk_mysql.NewMysqlBackend(&bk_mysql.MysqlSettings{
			Driver:         "mysql",
			DataSrouceName: "root:polechka2003@tcp(127.0.0.1)/test",
		})

		if err != nil {
			log.Error().Msgf("failed to initialize mysql backend: %s", err.Error())
			return nil
		}

		log.Info().Msgf("using backend: %s", bk.BackendTypeStr(settings.BackendType))

	case bk.BackendType_Memory: // never fails
		dbBackend = bk_memory.NewMemoryBackend()
		log.Info().Msgf("using backend: %s", bk.BackendTypeStr(settings.BackendType))

	default:
		log.Warn().Msgf("[%s] is not supported", bk.BackendTypeStr(settings.BackendType))
		dbBackend = bk_memory.NewMemoryBackend()
	}

	// lc := net.ListenConfig{} // for TLS connection
	listener, err := net.Listen(settings.NetworkProtocol, settings.Addr)
	if err != nil {
		log.Error().Msgf("failed to created a listener: %v", err.Error())
		listener.Close()
		return nil
	}

	// s.loadDataFromDB()

	return &Session{
		clients:            NewClientMap(),
		duration:           timeout,
		timer:              time.NewTimer(timeout),
		listener:           listener,
		network:            settings.NetworkProtocol,
		address:            settings.Addr,
		onSessionJoinCh:    make(chan *Client),
		onSessionLeaveCh:   make(chan string),
		clientMessagesCh:   make(chan *ClientMessage),
		sessionMessagesCh:  make(chan *SessionMessage),
		greetingMessagesCh: make(chan *GreetingMessage),
		quitCh:             make(chan struct{}),
		timeoutCh:          make(chan struct{}),
		backend:            dbBackend,
		chatHistory:        ChatHistory{},
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

func (s *Session) readClientName(conn net.Conn, receiver string) bool {
	const clientNameMaxBytes = 256
	s.sessionMessagesCh <- newSessionMsg(receiver, []byte("@name:"))

	buf := make([]byte, 4096)
	for {
		bytesRead, err := conn.Read(buf)
		if err != nil && err != io.EOF {
			log.Error().Msgf("failed to read from a connection: %s", conn.RemoteAddr().String())
			return false
		}

		// Opposite side closed the connection
		if bytesRead == 0 {
			return false
		}

		// Ideally, the validation has to be done on the client side,
		// but since this is a cli application, we have no choise other than
		// process it on the server side, when we receiver actual bytes.
		if bytesRead > clientNameMaxBytes {
			// Notify a client that the name was too long.
			contents := []byte(fmt.Sprintf("@name: [%s] exceeds 256 chars", buf[:bytesRead]))
			s.sessionMessagesCh <- newSessionMsg(receiver, contents)

			// Send a message to enter a different name.
			s.sessionMessagesCh <- newSessionMsg(receiver, []byte("@name:"))
			continue
		}

		clientName := string(common.StripCR(buf, bytesRead))

		// if s.backend.HasClient(expectedClientName) {
		// 	// send a message that a client already exists.
		// 	// Is it expensive to make a database query every time in a for loop?
		// 	// Maybe a better approach would be to cache all the clients on session's startup,
		// 	// maybe in a map, and then make a lookup in a map itself rather than making a query into the database.
		// } else {
		// 	// append client to a database
		// 	// s.backend.RegisterClient(expectedClientName, remoteAddr, "connected", time.Now())
		// }

		if s.clients.existsWithName(receiver, clientName) {
			// Notify a client that this name is already occupied by someone else,
			// and request to specify a different one.
			contents := []byte(fmt.Sprintf("@name: [%s] already exists", clientName))
			s.sessionMessagesCh <- newSessionMsg(receiver, contents)

			s.sessionMessagesCh <- newSessionMsg(receiver, []byte("@name:"))
		} else {
			// Update status from Pending to Connected.
			s.clients.updateStatus(receiver, ClientStatus_Connected)

			s.clients.assignName(receiver, clientName)

			log.Info().Msgf("client: [%s] was connected", clientName)

			return true
		}
	}
}

func (s *Session) handleConnection(conn net.Conn) {
	abortCh := make(chan struct{})
	quitCh := make(chan struct{})

	go disconnectIdleClient(conn, abortCh, quitCh)

	thisClient := &Client{
		conn:   conn,
		ipAddr: conn.RemoteAddr().String(),
		status: ClientStatus_Pending,
	}

	s.onSessionJoinCh <- thisClient

	if s.readClientName(conn, thisClient.ipAddr) {
		if s.nClients.Load() > 1 {
			// Notify all other clients about a new joiner
			// thisClient.name has to be used instead of its IP address,
			// since at this point it should be already available.
			contents := []byte(fmt.Sprintf("client: [%s] joined", thisClient.name))
			exclude := thisClient.ipAddr
			s.greetingMessagesCh <- newGreetingMsg(contents, exclude)

			// Display the list of all participants to this client
			s.listParticipants(conn, thisClient.ipAddr)

			// TODO: The history is not display correctly,
			// some messages are cut.

			// Display a chat history
			if s.chatHistory.size() != 0 {
				s.displayChatHistory(conn, thisClient.ipAddr)
			}
		}

		buf := make([]byte, 4096)
		for {
			nBytes, err := conn.Read(buf)
			if err != nil && err != io.EOF {
				// If the client was idle for too long, it will be automatically disconnected from the session.
				// Multiplexing is used to differentiate between an error, and us, who closed the connection intentionally.
				select {
				case <-quitCh:
					// Notify the client that it will be disconnected
					contents := []byte("you were idle for too long, disconnecting")
					s.sessionMessagesCh <- newSessionMsg(thisClient.ipAddr, contents)

					log.Info().Msgf("client: [%s]: has been idle for too long, disconnecting", thisClient.name)
				default:
					log.Error().Msgf("failed to read bytes from the client: %s", err.Error())
				}
				break
			}

			// An opposite side closed the connection,
			// break out from a for loop and send a client left message.
			if nBytes == 0 {
				break
			}

			// Client has receiver a message, abort disconnecting.
			abortCh <- struct{}{}

			// Would it be possible to serialize the message, send it as bytes,
			// and then deserialize it on the client side?
			// so, the message would have a header and a body
			// containing an actual contents.
			s.clientMessagesCh <- newClientMsg(thisClient.ipAddr, thisClient.name, buf[:nBytes])
		}
	}

	s.onSessionLeaveCh <- thisClient.ipAddr

	// Notify all the clients about who left the session.
	if s.nClients.Load() > 1 {
		contents := []byte(fmt.Sprintf("client: [%s] left", thisClient.name))
		exclude := thisClient.ipAddr
		s.greetingMessagesCh <- newGreetingMsg(contents, exclude)
	}

	conn.Close()
}

func (s *Session) processConnections() {
	s.running = true

	for s.running {
		select {
		case client := <-s.onSessionJoinCh:
			log.Info().Msgf("[%15s] joined the session", client.ipAddr)

			s.stats.ClientsJoined.Add(1)

			s.clients.add(client.ipAddr, client)
			s.nClients.Add(1)
			s.timer.Stop()

		case addr := <-s.onSessionLeaveCh:
			log.Info().Msgf("[%15s] left the session", addr)

			s.stats.ClientsLeft.Add(1)

			s.clients.updateStatus(addr, ClientStatus_Disconnected)
			s.nClients.Add(-1)

			// Reset the timer if all clients left the session.
			if s.nClients.Load() == 0 {
				s.timer.Reset(s.duration)
			}

		case msg := <-s.clientMessagesCh:
			log.Info().Msgf("received message: %s", string(msg.contents))

			// Add message to the chat history
			s.chatHistory.push(msg)

			s.stats.MessagesReceived.Add(1)

			clients := make([]*Client, s.clients.size())
			s.clients.getClients(clients)

			for _, client := range clients {
				if client.matchStatus(ClientStatus_Connected) {
					// How we can avoid string comparison?
					if strings.Compare(client.ipAddr, msg.senderIpAddr) != 0 {
						formatedMsg := []byte(fmt.Sprintf("[%s] [%s] %s", msg.sendTime.Format(time.DateTime), msg.senderName, string(msg.contents)))
						formatedMsgSize := int64(len(formatedMsg))
						bytesWritten, err := writeBytesToClient(client, formatedMsg, formatedMsgSize)
						if err != nil {
							log.Error().Msgf("failed to write bytes to a remote connection: [%s], error: %s", client.ipAddr, err.Error())
							continue
						}
						if bytesWritten == formatedMsgSize {
							s.stats.MessagesSent.Add(1)
						} else {
							s.stats.MessagesDropped.Add(1)
						}
					}
				}
			}

		case msg := <-s.sessionMessagesCh:
			// Broadcast the message to a particular receiver
			var client Client
			if s.clients.tryGet(msg.receiver, &client) {
				if !client.matchStatus(ClientStatus_Disconnected) {
					formatedMsg := []byte(fmt.Sprintf("[%s] %s", msg.sendTime.Format(time.DateTime), string(msg.contents)))
					formatedMsgSize := int64(len(formatedMsg))
					bytesWritten, err := writeBytesToClient(&client, formatedMsg, int64(formatedMsgSize))
					if err != nil {
						log.Error().Msgf("failed to write bytes to a remote connection: [%s], error: %s", client.ipAddr, err.Error())
					} else {
						if bytesWritten == formatedMsgSize {
							s.stats.MessagesSent.Add(1)
						}
					}
				}
			}

		case msg := <-s.greetingMessagesCh:
			// Greeting messages should be broadcasted to all the clients, except sender,
			// which is specified inside exclude list.
			clients := make([]*Client, s.clients.size())
			s.clients.getClients(clients)

			for _, client := range clients {
				if client.matchStatus(ClientStatus_Disconnected) {
					continue
				}

				// Skip the sender itself.
				if strings.Compare(client.ipAddr, msg.exclude) != 0 {
					formatedMsg := []byte(fmt.Sprintf("[%s] %s", msg.sendTime.Format(time.DateTime), string(msg.contents)))
					formatedMsgSize := int64(len(formatedMsg))
					bytesWritten, err := writeBytesToClient(client, formatedMsg, formatedMsgSize)
					if err != nil {
						log.Error().Msgf("failed to write bytes to a remote connection: [%s], error: %s", client.ipAddr, err.Error())
						break
					}
					if bytesWritten == formatedMsgSize {
						s.stats.MessagesSent.Add(1)
					}
				}
			}

		case <-s.timer.C:
			log.Info().Msg("session timeout, no clients joined.")

			s.running = false
			close(s.timeoutCh)
		}
	}
}

// Write bytes to a remote connection.
// Returns an amount of bytes successfully written and an error, if any.
func writeBytesToClient(client *Client, contents []byte, contentsSize int64) (int64, error) {
	var bytesWritten int64
	for bytesWritten < contentsSize {
		n, err := client.conn.Write(contents[bytesWritten:])
		if err != nil {
			return bytesWritten, err
		}
		bytesWritten += int64(n)
	}
	return bytesWritten, nil
}

// NOTE: My impression that it would be better to introduce inboxes and outboxes for the session
// as well, so we can better track down send/received messages (in one place),
// instead of calling conn.Write function everywhere in the code and trying to handle if it fails.
// But we would have to define a notion how do distinguish between different message types.
func (s *Session) listParticipants(conn net.Conn, receiverIpAddr string) {
	clients := make([]*Client, s.clients.size())
	s.clients.getClients(clients)

	sessionParticipants := "participants: "

	for _, client := range clients {
		if client.matchStatus(ClientStatus_Pending) {
			continue
		}

		status := "disconnected"
		if client.matchStatus(ClientStatus_Connected) {
			status, _ = strings.CutPrefix(status, "dis")
		}

		sessionParticipants += fmt.Sprintf("\n[%s] *%s", client.name, status)
	}

	s.sessionMessagesCh <- newSessionMsg(receiverIpAddr, []byte(sessionParticipants))
}

// When new client joins, we should display all available participants and a whole chat history.
// This whole function most likely should be protected with a mutex.
// conn is a new joiner.
func (s *Session) displayChatHistory(conn net.Conn, receiverIpAddr string) {
	messages := make([]ClientMessage, s.chatHistory.size())
	s.chatHistory.flush(messages)

	history := make([]string, s.chatHistory.size())
	for _, msg := range messages {
		fmtMsg := fmt.Sprintf("%s [%s]: %s", msg.sendTime.Format(time.DateTime), msg.senderName, string(msg.contents))
		history = append(history, fmtMsg)
	}

	s.sessionMessagesCh <- newSessionMsg(receiverIpAddr, []byte(strings.Join(history, "\n")))
}
