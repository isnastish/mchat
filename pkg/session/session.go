package session

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

const (
	RegisterClientOption     = "1"
	AuthenticateClientOption = "2"
	ShutDownClient           = "3"
	CreateNewChannel         = "4"
)

const (
	ConnState_ProcessingMenu = iota + 1
	ConnState_RegisteringNewClient
	ConnState_AuthenticatingClient
	ConnState_ProcessingClientMessages
	ConnState_CreatingChannel
)

const (
	ConnSubstate_None = iota

	// Substates for Registering/authenticating a client
	ConnSubstate_ReadingClientName
	ConnSubstate_ReadingClientPassword

	//  Substates for creating a channel
	ConnSubstate_ReadingChannelName
)

type ConnState struct {
	Conn           net.Conn
	State          int
	Substate       int
	ClientName     string
	ClientPassword string
	Input          string // TODO: Should be bytes []byte
}

type ChatHistory struct {
	messages []ClientMessage
	count    int32
	cap      int32
	mu       sync.Mutex
}

type Settings struct {
	NetworkProtocol string // tcp|udp
	Addr            string
	BackendType     int
}

type Session struct {
	clientsMap           map[string]*Client
	timer                *time.Timer
	duration             time.Duration
	timeoutCh            chan struct{}
	connectedClientCount atomic.Int32
	listener             net.Listener
	network              string // tcp|udp
	address              string
	onSessionJoinCh      chan *Client
	onSessionLeaveCh     chan string
	clientMessagesCh     chan *ClientMessage
	sessionMessagesCh    chan *SessionMessage
	greetingMessagesCh   chan *GreetingMessage
	running              bool
	quitCh               chan struct{}
	stats                sts.Stats // atomic
	backend              bk.DatabaseBackend
	chatHistory          ChatHistory
	mu                   sync.Mutex
	// When a session starts, we should upload all the client names from our backend database
	// into a runtimeDB, so we can operate on a map itself, rather than making queries to a database
	// when we need to verify user's name.
	// runtimeDB map[string]bool
}

// TODO: Pull out all global variables into its own struct
var clientStatusOnline = "online"
var clientStatusOffline = "offline"
var clientNameMsg = []byte("@name: ")
var clientPasswordMsg = []byte("@password: ")
var clientNameMaxLen = 64
var clientPasswordMaxLen = 256
var log = lgr.NewLogger("debug")
var menu = []string{
	"[1] - Register",
	"[2] - Log in",
	"[3] - Exit",

	// These are only available when the clients is alredy logged in,
	// And shouldn't be visible when we're about to log in.
	"[4] - CreateChannel (not implemented)",
	"[5] - List channels (not implemented)",
}

func (s *ConnState) Transition(newState int) {
	if (newState == ConnState_RegisteringNewClient ||
		newState == ConnState_AuthenticatingClient) &&
		s.Substate == ConnSubstate_ReadingClientName {
		// transition to processing the password
		s.Substate = ConnSubstate_ReadingClientPassword
	} else if newState == ConnState_CreatingChannel {
		// transition to processing channels name
		s.Substate = ConnSubstate_ReadingChannelName
	}
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

// type ClientMap struct {
// 	clientsList []*Client
// 	count       int32
// 	clientsMap  map[string]*Client
// 	mu          sync.Mutex
// }

// func NewClientMap() *ClientMap {
// 	return &ClientMap{
// 		clientsMap: make(map[string]*Client),
// 	}
// }

// func (m *ClientMap) Push(addr string, c *Client) {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()
// 	m.clientsMap[addr] = c
// 	m.clientsList = append(m.clientsList, c)
// 	m.count++
// }

// // addr has to be a name
// func (m *ClientMap) UpdateStatus(addr string, status int32) {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()
// 	m.clientsMap[addr].status = status
// }

// // TODO(alx): Rename to SetName
// func (m *ClientMap) AssignName(addr string, name string) {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()
// 	m.clientsMap[addr].name = name
// }

// func (m *ClientMap) SetPassword(addr string, password string) {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()
// 	m.clientsMap[addr].password = password
// }

// func (m *ClientMap) Size() int32 {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()
// 	return m.count
// }

// func (m *ClientMap) Flush(out []*Client) int32 {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()
// 	return int32(copy(out, m.clientsList))
// }

// func (m *ClientMap) TryGet(addr string, out *Client) bool {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()
// 	c, exists := m.clientsMap[addr]
// 	if exists {
// 		*out = *c
// 	}
// 	return exists
// }

// // This function will be removed when we add a database support.
// // So we make a query into a database instead of searching in a map.
// func (m *ClientMap) ExistsWithName(addr string, name string) bool {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()
// 	c, exists := m.clientsMap[addr]
// 	if exists && strings.Compare(c.name, name) == 0 {
// 		return true
// 	}
// 	return false
// }

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

	// lc := net.ListenConfig{} // for TLS connection
	listener, err := net.Listen(settings.NetworkProtocol, settings.Addr)
	if err != nil {
		log.Error().Msgf("failed to created a listener: %v", err.Error())
		listener.Close()
		return nil
	}

	// s.loadDataFromDB()

	return &Session{
		clientsMap:         make(map[string]*Client),
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
	go s.processMessages()

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
	defer conn.Close()

	newClient := &Client{
		Conn:     conn,
		IpAddr:   conn.RemoteAddr().String(),
		State:    ClientState_Pending,
		JoinTime: time.Now(),
	}

	s.mu.Lock()
	s.clientsMap[newClient.IpAddr] = newClient
	s.stats.ConnCount++
	s.mu.Unlock()

	// log.Info().Msgf("[%15s] connection discovered", client.ipAddr)
	// s.stats.ClientsJoined.Add(1)
	// s.clients.Push(client.ipAddr, client)
	// s.nClients.Add(1)
	// s.timer.Stop()

	// s.mu.Lock()
	// if client.state.match(ClientState_Pending) {
	// 	// Client left during Registration/ or connection process
	// 	// So we have to remove it, since he didn't finish the registration
	// 	delete(s.clients, client.ipAddr)
	// } else if
	// s.mu.Unlock()

	// log.Info().Msgf("[%15s] left the session", addr)
	// s.stats.ClientsLeft.Add(1)
	// s.clients.UpdateStatus(addr, ClientStatus_Disconnected)
	// s.nClients.Add(-1)
	// // Reset the timer if all clients left the session.
	// if s.nClients.Load() == 0 {
	// 	s.timer.Reset(s.duration)
	// }

	connState := ConnState{
		Conn:     conn,
		State:    ConnState_ProcessingMenu,
		Substate: ConnSubstate_None,
	}

	abortCh := make(chan struct{})
	quitConnCh := make(chan struct{})

	go disconnectIdleClient(conn, abortCh, quitConnCh)

	s.displayMenu(newClient)

	// listClientsAndChatHistory := func() {
	// 	contents := []byte(fmt.Sprintf("client: [%s] joined", thisClient.name))
	// 	exclude := thisClient.ipAddr
	// 	s.greetingMessagesCh <- NewGreetingMsg(contents, exclude)
	// 	s.listParticipants(conn, newClient.IpAddr)
	// 	if s.chatHistory.Size() != 0 {
	// 		s.displayChatHistory(conn, thisClient.ipAddr)
	// 	}
	// }

Loop:
	for {
		buffer := make([]byte, 4096)
		bytesRead, err := conn.Read(buffer)

		// TODO: I would make sense to make it be a part of a connection reader
		if err != nil && err != io.EOF {
			// If the client was idle for too long, it will be automatically disconnected from the session.
			// Multiplexing is used to differentiate between an error, and us, who closed the connection intentionally.
			select {
			case <-quitConnCh:
				// Notify the client that it will be disconnected
				contents := []byte("you were idle for too long, disconnecting")
				s.sessionMessagesCh <- NewSessionMsg(newClient, contents)
				log.Info().Msgf("client: [%s]: has been idle for too long, disconnecting", newClient.IpAddr)
			default:
				log.Error().Msgf("failed to read bytes from the client: %s", err.Error())
			}
			return
		}

		// If the opposite side closed the connection,
		// break out of loop.
		if bytesRead == 0 {
			break
		}

		connState.Input = string(common.StripCR(buffer, bytesRead))

		switch connState.State {
		case ConnState_ProcessingMenu:
			if strings.Compare(connState.Input, RegisterClientOption) == 0 {
				connState.Transition(ConnState_RegisteringNewClient)
			} else if strings.Compare(connState.Input, AuthenticateClientOption) == 0 {
				connState.Transition(ConnState_AuthenticatingClient)
			} else if strings.Compare(connState.Input, ShutDownClient) == 0 {
				break Loop
			} else if strings.Compare(connState.Input, CreateNewChannel) == 0 {
				connState.Transition(ConnState_CreatingChannel)
			}

		// TODO: Combine ConnState_RegisteringNewClient and ConnState_AuthenticatingClient in a single state
		// to avoid duplications
		case ConnState_RegisteringNewClient:
			if connState.Substate == ConnSubstate_ReadingClientName {
				if bytesRead > clientNameMaxLen {
					msg := []byte(fmt.Sprintf("name cannot exceed %d characters", clientNameMaxLen))
					s.sessionMessagesCh <- NewSessionMsg(newClient, msg)
					s.sessionMessagesCh <- NewSessionMsg(newClient, clientNameMsg)
				} else {
					if s.backend.HasClient(connState.Input) {
						msg := []byte("name is occupied, try a different one")
						s.sessionMessagesCh <- NewSessionMsg(newClient, msg)
						s.sessionMessagesCh <- NewSessionMsg(newClient, clientNameMsg)
					} else {
						connState.Transition(ConnState_RegisteringNewClient)
						connState.ClientName = connState.Input
					}
				}
			} else if connState.Substate == ConnSubstate_ReadingClientPassword {
				if bytesRead > clientPasswordMaxLen {
					msg := []byte(fmt.Sprintf("password cannot exceed %d characters", clientPasswordMaxLen))
					s.sessionMessagesCh <- NewSessionMsg(newClient, msg)
					s.sessionMessagesCh <- NewSessionMsg(newClient, clientPasswordMsg)
				} else {
					connState.Substate = ConnSubstate_None
					connState.State = ConnState_ProcessingClientMessages

					connState.ClientPassword = connState.Input

					s.mu.Lock()
					s.backend.RegisterNewClient(
						connState.ClientName,
						newClient.IpAddr,
						clientStatusOnline,
						connState.Input,
						newClient.JoinTime,
					)

					// Check whether a client with this name already exists and is disconnected.
					// Update its connection so we can read/write from it and change the ip address.
					client, exists := s.clientsMap[connState.ClientName]
					if exists && client.State == ClientState_Disconnected {
						// delete the connection
						delete(s.clientsMap, newClient.IpAddr)
					} else {
						panic("client is connected from a different machine")
					}

					client, exists = s.clientsMap[newClient.IpAddr]
					if exists && client.State == ClientState_Pending {
						// Rehash the client based on its name rather than ip address
						newClient.Name = connState.ClientName
						newClient.State = ClientState_Connected
						delete(s.clientsMap, newClient.IpAddr)
						s.clientsMap[newClient.Name] = newClient
					} else {
						panic("client has to be present in a map")
					}
					s.mu.Unlock()

					s.connectedClientCount.Add(1)

					// For greeting messages we can rely on client's name,
					// because it happens after the client has been connected.
					// Maybe we should print another menu here which allows the client to choose whether
					// to display a chat history or to list all participants
					if s.connectedClientCount.Load() > 1 {
						msg := []byte(fmt.Sprintf("client: [%s] joined", connState.ClientName))
						s.greetingMessagesCh <- NewGreetingMsg(newClient, msg)
						s.listParticipants(newClient)
						if s.chatHistory.Size() != 0 {
							history := make([]string, s.chatHistory.Size())
							msgs := make([]ClientMessage, s.chatHistory.Size())

							s.chatHistory.Flush(msgs)

							for _, msg := range msgs {
								m, _ := msg.Format()
								history = append(history, string(m))
							}
							s.sessionMessagesCh <- NewSessionMsg(newClient, []byte(strings.Join(history, "\n")))
						}
					}

					log.Info().Msgf("new client [%s] was registered", connState.ClientName)
				}
			}

		case ConnState_AuthenticatingClient:
			if connState.Substate == ConnSubstate_ReadingClientName {
				if bytesRead > clientNameMaxLen {
					msg := []byte(fmt.Sprintf("name cannot exceed %d characters", clientNameMaxLen))
					s.sessionMessagesCh <- NewSessionMsg(newClient, msg)
					s.sessionMessagesCh <- NewSessionMsg(newClient, clientNameMsg)
				} else {
					if !s.backend.HasClient(connState.Input) {
						msg := []byte(fmt.Sprintf("client with name [%s] doesn't exist"))
						s.sessionMessagesCh <- NewSessionMsg(newClient, msg)
						s.sessionMessagesCh <- NewSessionMsg(newClient, clientNameMsg)
					} else {
						connState.Transition(ConnState_AuthenticatingClient)
						connState.ClientName = connState.Input
					}
				}
			} else if connState.Substate == ConnSubstate_ReadingClientPassword {
				if bytesRead > clientPasswordMaxLen {
					msg := []byte(fmt.Sprintf("password cannot exceed %d characters", clientPasswordMaxLen))
					s.sessionMessagesCh <- NewSessionMsg(newClient, msg)
					s.sessionMessagesCh <- NewSessionMsg(newClient, clientPasswordMsg)
				} else {
					connState.ClientPassword = connState.Input

					if !s.backend.MatchClientPassword(connState.ClientName, connState.ClientPassword) {
						msg := []byte("password isn't correct, try again")
						s.sessionMessagesCh <- NewSessionMsg(newClient, msg)
						s.sessionMessagesCh <- NewSessionMsg(newClient, clientPasswordMsg)
					} else {
						connState.Substate = ConnSubstate_None
						connState.State = ConnState_ProcessingClientMessages

						connState.ClientPassword = connState.Input

						s.mu.Lock()
						client, exists := s.clientsMap[connState.ClientName]
						if exists && client.State == ClientState_Disconnected {
							delete(s.clientsMap, connState.ClientName)
						} else {
							panic("client connected from a different machine")
						}

						client, exists = s.clientsMap[newClient.IpAddr]
						if exists && client.State == ClientState_Pending {
							// Rehash new client
							newClient.State = ClientState_Connected
							newClient.Name = connState.ClientName
							delete(s.clientsMap, newClient.IpAddr)
							s.clientsMap[newClient.Name] = newClient
						} else {
							panic("client has to exist with a Pending status")
						}
						s.mu.Unlock()
						s.connectedClientCount.Add(1)

						if s.connectedClientCount.Load() > 1 {
							msg := []byte(fmt.Sprintf("client: [%s] joined", connState.ClientName))
							s.greetingMessagesCh <- NewGreetingMsg(newClient, msg)
							// listClientsAndChatHistory()
						}

						log.Info().Msgf("client [%s] was authenticated", connState.ClientName)
					}
				}
			}

		case ConnState_ProcessingClientMessages:
			s.clientMessagesCh <- NewClientMsg(newClient.IpAddr, newClient.Name, buffer[:bytesRead])

		case ConnState_CreatingChannel:
			panic("Creating channels is not supported!")
		}

		// Client has received a message, thus is not idle,
		// abort disconnection.
		abortCh <- struct{}{}
	}

	s.mu.Lock()
	if newClient.State == ClientState_Connected {
		client, exists := s.clientsMap[newClient.Name]
		if exists {
			client.State = ClientState_Disconnected
		}
		if s.connectedClientCount.Load() > 1 {
			msg := []byte(fmt.Sprintf("client: [%s] left", newClient.Name))
			s.greetingMessagesCh <- NewGreetingMsg(newClient, msg)
		}
		s.connectedClientCount.Add(-1)
	}
	s.mu.Unlock()
}

func (s *Session) processMessages() {
	s.running = true

	for s.running {
		select {
		case msg := <-s.clientMessagesCh:
			log.Info().Msgf("received client's message: %s", string(msg.Contents))

			s.chatHistory.Push(msg)
			s.stats.MessagesReceived.Add(1)

			s.mu.Lock()
			for _, client := range s.clientsMap {
				if client.State == ClientState_Connected {
					// TODO: Is it possible to avoid string comparison for each client?
					// We perform the comparison in order not to send a message to excluded client.
					if strings.Compare(client.Name, msg.SenderName) != 0 {
						fMsg, fMsgSize := msg.Format()
						bWritten, err := writeBytesToClient(client, fMsg, fMsgSize)

						if err != nil || bWritten != fMsgSize {
							log.Error().Msgf("failed to write bytes to [%s], error %s", client.Name, err.Error())
						} else {
							s.stats.MessagesSent.Add(1)
						}
					}
				}
			}
			s.mu.Unlock()

		case msg := <-s.sessionMessagesCh:
			log.Info().Msgf("received session's message: %s", string(msg.Contents))

			// The recipient's state can be either ClientState_Pending or ClientState_Connected
			s.mu.Lock()

			fMsg, fMsgSize := msg.Format()
			bWritten, err := writeBytesToClient(msg.Recipient, fMsg, fMsgSize)

			if err != nil || bWritten != fMsgSize {
				log.Error().Msgf("failed to write bytes to [%s], error %s", msg.Recipient.IpAddr, err.Error())
			} else {
				s.stats.MessagesSent.Add(1)
			}
			s.mu.Unlock()

		case msg := <-s.greetingMessagesCh:
			// Greeting messages should be broadcasted to all the clients, except sender,
			// which is specified inside exclude list.
			log.Info().Msgf("received greeting message: %s", string(msg.Contents))

			// TODO: For large amount of clients this has to be done in parallel,
			// because currently the running cost is O(n).
			// We can split the map into different subchunks and process them in a separate go routines.
			s.mu.Lock()
			for _, client := range s.clientsMap {
				if client.State == ClientState_Connected {
					if strings.Compare(client.Name, msg.Exclude.Name) != 0 {
						fMsg, fMsgSize := msg.Format()
						bWritten, err := writeBytesToClient(client, fMsg, fMsgSize)

						if err != nil || bWritten != fMsgSize {
							log.Error().Msgf("failed to write bytes to [%s], error %s", client.Name, err.Error())
						} else {
							s.stats.MessagesSent.Add(1)
						}
					}
				}
			}
			s.mu.Unlock()

		case <-s.timer.C:
			log.Info().Msg("session timeout, no clients joined.")

			s.running = false
			close(s.timeoutCh)
		}
	}
}

// Write bytes to a remote connection.
// Returns an amount of bytes successfully written and an error, if any.
func writeBytesToClient(client *Client, contents []byte, contentsSize int) (int, error) {
	var bWritten int
	for bWritten < contentsSize {
		n, err := client.Conn.Write(contents[bWritten:])
		if err != nil {
			return bWritten, err
		}
		bWritten += n
	}
	return bWritten, nil
}

// NOTE: My impression that it would be better to introduce inboxes and outboxes for the session
// as well, so we can better track down send/received messages (in one place),
// instead of calling conn.Write function everywhere in the code and trying to handle if it fails.
// But we would have to define a notion how do distinguish between different message types.
// Maybe we have protect the whole struct with a mutex. What if the size changes?
func (s *Session) listParticipants(recipient *Client) {
	var state string
	var participants []string

	s.mu.Lock()
	for _, client := range s.clientsMap {
		switch client.State {
		case ClientState_Connected:
			state = clientStatusOnline
		case ClientState_Disconnected:
			state = clientStatusOffline
		default:
			if client.State == ClientState_Pending {
				continue
			}
		}
		participants = append(participants, fmt.Sprintf("\t[%s] *%s", client.Name, state))
	}
	s.mu.Unlock()

	result := strings.Join(participants, "\n")
	result = "participants:\n" + result

	s.sessionMessagesCh <- NewSessionMsg(recipient, []byte(result), true)
}

// // When new client joins, we should display all available participants and a whole chat history.
// // This whole function most likely should be protected with a mutex.
// // conn is a new joiner.
// func (s *Session) displayChatHistory(recipient *Client) {
// 	messages := make([]ClientMessage, s.chatHistory.Size())
// 	s.chatHistory.Flush(messages)

// 	history := make([]string, s.chatHistory.Size())
// 	for _, msg := range messages {
// 		history = append(history, msg.Format())
// 	}
// 	s.sessionMessagesCh <- NewSessionMsg(recipient, []byte(strings.Join(history, "\n")))
// }

func (s *Session) displayMenu(recipient *Client) {
	const ignoreTime = true

	options := make([]string, len(menu))
	for i := 0; i < len(menu); i++ {
		options = append(options, fmt.Sprintf("\t%s\n", menu[i]))
	}
	menu := "Menu:\n" + strings.Join(options, "")
	s.sessionMessagesCh <- NewSessionMsg(recipient, []byte(menu), ignoreTime)
}
