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

	"github.com/isnastish/chat/pkg/common"
	lgr "github.com/isnastish/chat/pkg/logger"
	bk "github.com/isnastish/chat/pkg/session/backend"
	bk_memory "github.com/isnastish/chat/pkg/session/backend/memory"
	bk_mysql "github.com/isnastish/chat/pkg/session/backend/mysql"
	bk_redis "github.com/isnastish/chat/pkg/session/backend/redis"
	sts "github.com/isnastish/chat/pkg/stats"
)

// main states
const (
	ConnReader_ProcessingMenu_State = iota + 1
	ConnReader_RegisteringNewClient_State
	ConnReader_AuthenticatingClient_State
	ConnReader_ProcessingClientMessages_State
	ConnReader_CreatingChannel_State // not supported yet
)

// substates
const (
	None_SubState = iota
	ConnReader_ReadingClientsName_SubState
	ConnReader_ReadingClientsPassword_SubState
)

type ConnReader struct {
	conn     net.Conn
	state    int
	substate int
}

type Settings struct {
	NetworkProtocol string // tcp|udp
	Addr            string
	BackendType     int
}

type ChatHistory struct {
	messages []ClientMessage
	count    int32
	cap      int32
	mu       sync.Mutex
}

var CLIENT_NAME_MAX_LEN = 64
var CLIENT_PASSWORD_MAX_LEN = 256
var DEBUG_ENABLED = true
var log = lgr.NewLogger("debug")
var menu = []string{
	"[1] - Register",
	"[2] - Log in",
	"[3] - Exit",
	"[4] - CreateChannel (not implemented)", // this should be available after we've logged in
	"[5] - List channels (not implemented)",
}

func (h *ChatHistory) Push(msg *ClientMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.count >= h.cap {
		new_cap := max(h.cap+1, h.cap<<2)
		new_messages := make([]ClientMessage, new_cap)
		copy(new_messages, h.messages)
		h.messages = new_messages
		h.cap = new_cap
	}
	h.messages[h.count] = *msg
	h.count++
}

func (h *ChatHistory) Size() int32 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.count
}

func (h *ChatHistory) Flush(out []ClientMessage) int32 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return int32(copy(out, h.messages))
}

type ClientMap struct {
	clientsList []*Client
	count       int32
	clientsMap  map[string]*Client
	mu          sync.Mutex
}

func NewClientMap() *ClientMap {
	return &ClientMap{
		clientsMap: make(map[string]*Client),
	}
}

func (m *ClientMap) Push(addr string, c *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clientsMap[addr] = c
	m.clientsList = append(m.clientsList, c)
	m.count++
}

// addr has to be a name
func (m *ClientMap) UpdateStatus(addr string, status int32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clientsMap[addr].status = status
}

// TODO(alx): Rename to SetName
func (m *ClientMap) AssignName(addr string, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clientsMap[addr].name = name
}

func (m *ClientMap) SetPassword(addr string, password string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clientsMap[addr].password = password
}

func (m *ClientMap) Size() int32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.count
}

func (m *ClientMap) Flush(out []*Client) int32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return int32(copy(out, m.clientsList))
}

func (m *ClientMap) TryGet(addr string, out *Client) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, exists := m.clientsMap[addr]
	if exists {
		*out = *c
	}
	return exists
}

// This function will be removed when we add a database support.
// So we make a query into a database instead of searching in a map.
func (m *ClientMap) ExistsWithName(addr string, name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, exists := m.clientsMap[addr]
	if exists && strings.Compare(c.name, name) == 0 {
		return true
	}
	return false
}

type Session struct {
	clients            *ClientMap
	pendingClients     map[string]*Client
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

	case bk.BackendType_Memory:
		dbBackend = bk_memory.NewMemBackend()
		log.Info().Msgf("using backend: %s", bk.BackendTypeStr(settings.BackendType))

	default:
		log.Warn().Msgf("[%s] is not supported", bk.BackendTypeStr(settings.BackendType))
		dbBackend = bk_memory.NewMemBackend()
	}

	lc := net.ListenConfig{} // for TLS connection
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

func (s *Session) handleConnection(conn net.Conn) {
	connReader := ConnReader{
		conn:     conn,
		state:    ConnReader_ProcessingMenu_State,
		substate: None_SubState,
	}

	abortCh := make(chan struct{})
	quitCh := make(chan struct{})

	go disconnectIdleClient(conn, abortCh, quitCh)

	thisClient := &Client{
		conn:   conn,
		ipAddr: conn.RemoteAddr().String(),
		status: ClientStatus_Pending,
	}

	s.onSessionJoinCh <- thisClient

	s.displayMenu(conn, thisClient.ipAddr)

	var clientName string

	listClientsAndChatHistory := func() {
		contents := []byte(fmt.Sprintf("client: [%s] joined", thisClient.name))
		exclude := thisClient.ipAddr
		s.greetingMessagesCh <- NewGreetingMsg(contents, exclude)
		s.listParticipants(conn, thisClient.ipAddr)
		if s.chatHistory.Size() != 0 {
			s.displayChatHistory(conn, thisClient.ipAddr)
		}
	}

Loop:
	for {
		buffer := make([]byte, 4096)
		bytesRead, err := conn.Read(buffer)

		// This should be a part of connection reader
		if err != nil && err != io.EOF {
			// If the client was idle for too long, it will be automatically disconnected from the session.
			// Multiplexing is used to differentiate between an error, and us, who closed the connection intentionally.
			select {
			case <-quitCh:
				// Notify the client that it will be disconnected
				contents := []byte("you were idle for too long, disconnecting")
				s.sessionMessagesCh <- NewSessionMsg(thisClient.ipAddr, contents)

				log.Info().Msgf("client: [%s]: has been idle for too long, disconnecting", thisClient.name)
			default:
				log.Error().Msgf("failed to read bytes from the client: %s", err.Error())
			}
			break
		}

		// An opposite side closed the connection,
		// break out from a for loop and send a client left message.
		if bytesRead == 0 {
			break
		}

		input := string(common.StripCR(buffer, bytesRead))

		switch connReader.state {
		case ConnReader_ProcessingMenu_State:
			if strings.Compare(input, "1") == 0 { // TODO: Introduce names for commands, so we don't compare against raw strings
				connReader.state = ConnReader_RegisteringNewClient_State
			} else if strings.Compare(input, "2") == 0 {
				connReader.state = ConnReader_AuthenticatingClient_State
			} else if strings.Compare(input, "3") == 0 {
				break Loop
			} else if strings.Compare(input, "4") == 0 {
				connReader.state = ConnReader_CreatingChannel_State
				connReader.substate = ConnReader_ReadingClientsName_SubState
			} else {
				// don't change the state, just send a warning message that such command doesn't exist
			}
		case ConnReader_RegisteringNewClient_State:
			if connReader.substate == ConnReader_ReadingClientsName_SubState {
				if bytesRead > CLIENT_NAME_MAX_LEN {
					msg := []byte(fmt.Sprintf("name cannot exceed %d characters", CLIENT_NAME_MAX_LEN))
					s.sessionMessagesCh <- NewSessionMsg(thisClient.ipAddr, msg)
					s.sessionMessagesCh <- NewSessionMsg(thisClient.ipAddr, []byte("@name: ")) // TODO(alx): replace with variable
				} else {
					if s.backend.HasClient(input) {
						msg := []byte("Name is occupied, try a different one")
						s.sessionMessagesCh <- NewSessionMsg(thisClient.ipAddr, msg)
						s.sessionMessagesCh <- NewSessionMsg(thisClient.ipAddr, []byte("@name: ")) // TODO(alx): replace with variable
					} else {
						connReader.substate = ConnReader_ReadingClientsPassword_SubState

						// save client name
						clientName = input

						// STUDY: Use pending map instead of s.clients, and once we retrieved all the credentials,
						// we can put it into s.clients map and in a database.
						// We should use IP address as a key in a hash map, use name instead,
						// because how do we check whether clients exists?
						// mu := sync.Mutex{}
						// mu.Lock()
						var client Client
						s.clients.TryGet(thisClient.ipAddr, &client)
						client.SetName(input)
						// mu.Unlock()
					}
				}
			} else if connReader.substate == ConnReader_ReadingClientsPassword_SubState {
				if bytesRead > CLIENT_PASSWORD_MAX_LEN {
					warningContents := []byte(fmt.Sprintf("password cannot exceed %d characters", CLIENT_PASSWORD_MAX_LEN))
					s.sessionMessagesCh <- NewSessionMsg(thisClient.ipAddr, warningContents)
					s.sessionMessagesCh <- NewSessionMsg(thisClient.ipAddr, []byte("@password: "))
				} else {
					connReader.substate = None_SubState
					connReader.state = ConnReader_ProcessingClientMessages_State

					// Insert a new client into a backing storage
					// `input` is our password, but most likely we would have to encode it using base64,
					// so we don't store raw data in a storage
					s.backend.RegisterNewClient(clientName, thisClient.ipAddr, "online", input, thisClient.joinTime)

					var client Client
					s.clients.TryGet(thisClient.ipAddr, &client)
					log.Info().Msgf("client [%s] was registered", client.name)

					// Notify all participants in a session about new joiner.
					// List participants list and a chat history to a newly arrived client.
					if s.nClients.Load() > 1 {
						listClientsAndChatHistory()
					}
				}
			}

		case ConnReader_AuthenticatingClient_State: // log in
			if connReader.substate == ConnReader_ReadingClientsName_SubState {
				if bytesRead > CLIENT_NAME_MAX_LEN {
					msg := []byte(fmt.Sprintf("name cannot exceed %d characters", CLIENT_NAME_MAX_LEN))
					s.sessionMessagesCh <- NewSessionMsg(thisClient.ipAddr, msg)
					s.sessionMessagesCh <- NewSessionMsg(thisClient.ipAddr, []byte("@name: ")) // TODO(alx): replace with variable
				} else {
					if !s.backend.HasClient(input) {
						msg := []byte(fmt.Sprintf("client with name [%s] doesn't exist"))
						s.sessionMessagesCh <- NewSessionMsg(thisClient.ipAddr, msg)
						s.sessionMessagesCh <- NewSessionMsg(thisClient.ipAddr, []byte("@name: "))
					} else {
						// make a state transition to processing password
						connReader.substate = ConnReader_ReadingClientsPassword_SubState

						// save client's name
						clientName = input
					}
				}
			} else if connReader.substate == ConnReader_ReadingClientsPassword_SubState {
				if bytesRead > CLIENT_PASSWORD_MAX_LEN {
					// TODO: Collapse into a function
					msg := []byte(fmt.Sprintf("password cannot exceed %d characters", CLIENT_PASSWORD_MAX_LEN))
					s.sessionMessagesCh <- NewSessionMsg(thisClient.ipAddr, msg)
					s.sessionMessagesCh <- NewSessionMsg(thisClient.ipAddr, []byte("@password: "))
				} else {
					if !s.backend.MatchClientPassword(clientName, input) {
						// prompt client to enter a new password
						msg := []byte("password isn't correct")
						s.sessionMessagesCh <- NewSessionMsg(thisClient.ipAddr, msg)
						s.sessionMessagesCh <- NewSessionMsg(thisClient.ipAddr, []byte("@password: "))
					} else {
						connReader.substate = None_SubState
						connReader.state = ConnReader_ProcessingClientMessages_State

						// Notify all participants in a session about new joiner.
						// List participants list and a chat history to a newly arrived client.
						if s.nClients.Load() > 1 {
							listClientsAndChatHistory()
						}
					}
				}
			}

		case ConnReader_ProcessingClientMessages_State:
			s.clientMessagesCh <- NewClientMsg(thisClient.ipAddr, thisClient.name, buffer[:bytesRead])

		case ConnReader_CreatingChannel_State:
			panic("Creating channels is not supported!")
		}

		// Client has receiver a message, abort disconnecting.
		abortCh <- struct{}{}
	}

	s.onSessionLeaveCh <- thisClient.ipAddr

	// Notify all the clients about who left the session.
	if s.nClients.Load() > 1 {
		contents := []byte(fmt.Sprintf("client: [%s] left", thisClient.name))
		exclude := thisClient.ipAddr
		s.greetingMessagesCh <- NewGreetingMsg(contents, exclude)
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

			s.clients.Push(client.ipAddr, client)
			s.nClients.Add(1)
			s.timer.Stop()

		case addr := <-s.onSessionLeaveCh:
			log.Info().Msgf("[%15s] left the session", addr)

			s.stats.ClientsLeft.Add(1)

			s.clients.UpdateStatus(addr, ClientStatus_Disconnected)
			s.nClients.Add(-1)

			// Reset the timer if all clients left the session.
			if s.nClients.Load() == 0 {
				s.timer.Reset(s.duration)
			}

		case msg := <-s.clientMessagesCh:
			log.Info().Msgf("received message: %s", string(msg.contents))

			// Add message to the chat history
			s.chatHistory.Push(msg)

			if DEBUG_ENABLED {
				size := s.chatHistory.Size()
				msgs := make([]ClientMessage, size)
				nCopied := s.chatHistory.Flush(msgs)
				log.Info().Msgf("messages copied: %d", nCopied)
				for _, msg := range msgs {
					log.Info().Msg(msg.format())
				}
			}

			s.stats.MessagesReceived.Add(1)

			clients := make([]*Client, s.clients.Size())
			s.clients.Flush(clients)

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
			if s.clients.TryGet(msg.receiver, &client) {
				if !client.matchStatus(ClientStatus_Disconnected) {
					formatedMsg := msg.format()
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
			clients := make([]*Client, s.clients.Size())
			s.clients.Flush(clients)

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
// Maybe we have protect the whole struct with a mutex. What if the size changes?
func (s *Session) listParticipants(conn net.Conn, receiverIpAddr string) {
	size := s.clients.Size()
	clients := make([]*Client, size)
	p := make([]string, size)

	s.clients.Flush(clients)

	for _, client := range clients {
		if client.matchStatus(ClientStatus_Pending) {
			continue
		}
		var status string
		if client.matchStatus(ClientStatus_Connected) {
			status = "online"
		} else if client.matchStatus(ClientStatus_Disconnected) {
			status = "offline"
		}
		p = append(p, fmt.Sprintf("\t[%s] *%s", client.name, status))
	}
	participants := strings.Join(p, "\n")
	participants = "participants:\n" + participants
	s.sessionMessagesCh <- NewSessionMsg(receiverIpAddr, []byte(participants), true)
}

// When new client joins, we should display all available participants and a whole chat history.
// This whole function most likely should be protected with a mutex.
// conn is a new joiner.
func (s *Session) displayChatHistory(conn net.Conn, receiverIpAddr string) {
	messages := make([]ClientMessage, s.chatHistory.Size())
	s.chatHistory.Flush(messages)

	history := make([]string, s.chatHistory.Size())
	for _, msg := range messages {
		history = append(history, msg.format())
	}
	s.sessionMessagesCh <- NewSessionMsg(receiverIpAddr, []byte(strings.Join(history, "\n")))
}

func (s *Session) displayMenu(conn net.Conn, receiverIpAddr string) {
	const ignoreTime = true

	options := make([]string, len(menu))

	for i := 0; i < len(menu); i++ {
		options = append(options, fmt.Sprintf("\t%s\n", menu[i]))
	}

	menu := "Menu:\n" + strings.Join(options, "")
	s.sessionMessagesCh <- NewSessionMsg(receiverIpAddr, []byte(menu), ignoreTime)
}
