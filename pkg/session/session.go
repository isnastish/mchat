package session

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/isnastish/chat/pkg/common"
	lgr "github.com/isnastish/chat/pkg/logger"
	backend "github.com/isnastish/chat/pkg/session/backend"
	dynamodb_backend "github.com/isnastish/chat/pkg/session/backend/dynamodb"
	memory_backend "github.com/isnastish/chat/pkg/session/backend/memory"
	redis_backend "github.com/isnastish/chat/pkg/session/backend/redis"
	messages "github.com/isnastish/chat/pkg/session/messages"
	sts "github.com/isnastish/chat/pkg/stats"
)

// (initial state) -> (transitions to state)
//
// MenuOptionType_RegisterClient     -> ConnReaderState_RegisteringClient
// MenuOptionType_AuthenticateClient -> ConnReaderState_AuthenticatingClient
// MenuOptionType_ShutDownClient -> None
// MenuOptionType_CreateChannel -> ConnReaderState_CreatingChannel
//
// ConnReaderState_RegisteringClient    -> ConnReaderSubstate_ReadingClientName
//    ConnReaderSubstate_ReadingClientName -> ConnReaderSubstate_ReadingClientPassword
// ConnReaderState_AuthenticatingClient -> ConnReaderSubstate_ReadingClientName
//    ConnReaderSubstate_ReadingClientName -> ConnReaderSubstate_ReadingClientPassword

// Unrelated to connection reader FSM
const (
	MenuOptionType_RegisterClient = iota + 1
	MenuOptionType_AuthenticateClient
	MenuOptionType_ShutDownClient
	MenuOptionType_CreateChannel
)

type ConnReaderState int32

const (
	ConnReaderState_ProcessingMenu ConnReaderSubstate = iota + 1
	ConnReaderState_RegisteringClient
	ConnReaderState_AuthenticatingClient
	ConnReaderState_ProcessingClientMessages
	ConnReaderState_CreatingChannel
)

type ConnReaderSubstate int32

const (
	ConnReaderSubstate_ReadingClientName ConnReaderSubstate = iota + 1
	ConnReaderSubstate_ReadingClientPassword
)

// TODO: Connection reader fsm should be autonomous,
// meaning that it should manager the state on its own
// and transition to a new state based on bytes read from the client.
type ConnReaderFSM struct {
	conn                           net.Conn
	curState                       ConnReaderState
	curSubstate                    ConnReaderSubstate
	associatedClientName           string
	associatedClientPasswordSha256 string
	input                          []byte
}

func matchState(oldState, newState ConnReaderState) bool {
	return oldState == newState
}

func mathSubstate(oldSubstate, newSubstate ConnReaderSubstate) bool {
	return oldSubstate == newSubstate
}

func (r *ConnReaderFSM) transition(newState ConnReaderState, newSubstate ConnReaderSubstate) bool {
	if matchState(r.curState, ConnReaderState_ProcessingMenu) {
		if matchState(newState, ConnReaderState_RegisteringClient) ||
			matchState(newState, ConnReaderState_AuthenticatingClient) {
			// The substate could either transition to ReadingClientNam
		}
	}
}

type SessionSettings struct {
	NetworkProtocol string
	Addr            string
	BackendType     backend.BackendType
}

type Session struct {
	clients            *ClientsMap // map[string]*Client // TODO: Protect clients map with a mutex (maybe renaming it to connMap)
	timer              *time.Timer
	duration           time.Duration
	timeoutCh          chan struct{}
	connectionsCount   atomic.Int32
	listener           net.Listener
	network            string // tcp|udp
	address            string
	onSessionJoinCh    chan *Client
	onSessionLeaveCh   chan string
	clientMessagesCh   chan *messages.ClientMessage
	sessionMessagesCh  chan *messages.SessionMessage
	greetingMessagesCh chan *messages.GreetingMessage
	running            bool
	quitCh             chan struct{}
	stats              sts.Stats // atomic
	// TODO: We need to protect the storage with a mutex.
	// And we would have to protect clientsMap with a mutex as well.
	backend     backend.DatabaseBackend
	chatHistory MessageHistory
	// When a session starts, we should upload all the client names from our backend database
	// into a runtimeDB, so we can operate on a map itself, rather than making queries to a database
	// when we need to verify user's name.
	// runtimeDB map[string]bool
	loggingEnabled     bool // shouldn't be atomic, only reads are performed
	onlineClientsCount atomic.Int32
}

// TODO: Pull out all global variables into its own struct
const clientNameMaxLen = 64
const clientPasswordMaxLen = 256

var submitClientNamePrompt = []byte("@name: ")
var submitClientPasswordPrompt = []byte("@password: ")

var menu = []string{
	"[1] - Register",
	"[2] - Log in",
	"[3] - Exit",

	// These are only available when the clients is alredy logged in,
	// And shouldn't be visible when we're about to log in.
	"[4] - CreateChannel (not implemented)",
	"[5] - List channels (not implemented)",
}
var log = lgr.NewLogger("debug")

func NewSession(settings *SessionSettings) *Session {
	const timeout = 10000 * time.Millisecond

	var backend obackend.DatabaseBackend
	var err error

	switch settings.BackendType {
	case BackendType_Redis:
		dbBackend, err = redis.NewRedisBackend(&bk_redis.RedisSettings{
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
		clients:            newMap(),
		duration:           timeout,
		timer:              time.NewTimer(timeout),
		listener:           listener,
		network:            settings.NetworkProtocol,
		address:            settings.Addr,
		onSessionJoinCh:    make(chan *Client),
		onSessionLeaveCh:   make(chan string),
		clientMessagesCh:   make(chan *messages.ClientMessage),
		sessionMessagesCh:  make(chan *messages.SessionMessage),
		greetingMessagesCh: make(chan *messages.GreetingMessage),
		quitCh:             make(chan struct{}),
		timeoutCh:          make(chan struct{}),
		backend:            dbBackend,
		chatHistory:        MessageHistory{},
	}
}

func (s *Session) Run() {
	go s.processMessages()

	log.Info().Msgf("accepting connections: %s", s.listener.Addr().String())

	go func() {
		<-s.timeoutCh
		close(s.quitCh)
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

// TODO: Specify a time duration programmatically so it can be used in
func (s *Session) disconnectIfClientWasIdleForTooLong(conn net.Conn, onIdleClientDisconnectedCh, abortDisconnectingClientCh, onClientErrorCh chan struct{}) {
	const duration = 5 * 60000 * time.Millisecond
	timeout := time.NewTimer(duration)

	for {
		select {
		// If a program has already received a value from t.C,
		// the timer is known to have expired and the channel drained.
		case <-timeout.C:
			// After closing the connection
			// any blocked Read or Write operations will be unblocked and return errors.
			close(onIdleClientDisconnectedCh)
			conn.Close() // TODO: We have a defer conn.Close() statement inside handleConnection already
			return
		case <-abortDisconnectingClientCh:
			if !timeout.Stop() {
				<-timeout.C
			}
			wasActive := timeout.Reset(duration)
			if s.loggingEnabled {
				fmt.Println("wasActive?: ", wasActive)
			}

		case <-onClientErrorCh:
			if s.loggingEnabled {
				log.Info().Msg("the client closed the connection forcefully.")
			}
			return
		}
	}
}

func (s *Session) disconnectIdleClients() {

}

func (s *Session) handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
	}()

	connReader := &ConnReader{
		conn:  conn,
		state: ConnReaderState(ConnReaderState_ProcessingMenu),
	}

	client := &Client{
		conn:     conn,
		ipAddr:   conn.RemoteAddr().String(),
		state:    ClientState_Pending,
		joinTime: time.Now(),
	}

	// Update stats
	s.connectionsCount.Add(1)

	// Preemptively add client to the clients map
	s.clients.addClient(client.ipAddr, client)

	// A connection has been discovered, stop session's timeout
	s.timer.Stop()

	onIdleClientDisconnectedCh := make(chan struct{})
	abortDisconnectingClientCh := make(chan struct{})
	onClientErrorCh := make(chan struct{})

	s.displayMenu(client)

	// listClientsAndChatHistory := func() {
	// 	contents := []byte(fmt.Sprintf("client: [%s] joined", thisClient.name))
	// 	exclude := thisClient.ipAddr
	// 	s.greetingMessagesCh <- NewGreetingMsg(contents, exclude)
	// 	s.listParticipants(conn, newClient.ipAddr)
	// 	if s.chatHistory.Size() != 0 {
	// 		s.displayChatHistory(conn, thisClient.ipAddr)
	// 	}
	// }

	readBuffer := make([]byte, 4096)

Loop:
	for {
		// TODO: This should be a part of a connection reader.
		bytesRead, err := conn.Read(readBuffer)
		if err != nil && err != io.EOF {
			select {
			case <-onIdleClientDisconnectedCh:
				s.sessionMessagesCh <- messages.NewSessionMsg(
					client,
					[]byte("you were idle for too long, disconnecting"),
				)
			default:
				close(onClientErrorCh)
				break Loop
			}
		}

		// When the opposite side closed the connection.
		if bytesRead == 0 {
			close(onClientErrorCh)
			break
		}

		connReader.input = common.StripCR(readBuffer, bytesRead)

		switch connReader.state {
		case ConnReaderState(ConnReaderState_ProcessingMenu):
			menuOption, err := strconv.Atoi(string(connReader.input))
			if err != nil {

			} else {
				switch menuOption {
				case MenuOptionType_RegisterClient:
					connReader.transition(ConnReaderState_RegisteringClient)
					// connReader.state = ConnReaderState(ConnReaderState_RegisteringClient)
					// connReader.substate = ConnReaderSubstate_ReadingClientName
					s.sessionMessagesCh <- messages.NewSessionMsg(client, submitClientNamePrompt)

				case MenuOptionType_AuthenticateClient:
					connReader.state = ConnReaderState(ConnReaderState_AuthenticatingClient)
					// connReader.substate = ConnSubstate_ReadingClientName
					s.sessionMessagesCh <- NewSessionMsg(newClient, clientNameMsg)

				case ShutDownClient:
					s.sessionMessagesCh <- NewSessionMsg(newClient, []byte("Exiting..."))
					break Loop

				case CreateNewChannel:
					connState.state = ConnState_CreatingChannel
					connState.substate = ConnSubstate_ReadingChannelName

				default:
					msg := []byte(fmt.Sprintf("Command [%s] is undefined", connState.input))
					s.sessionMessagesCh <- NewSessionMsg(newClient, msg)
					s.displayMenu(newClient)
				}
			}

		// TODO: Combine ConnState_RegisteringNewClient and ConnState_AuthenticatingClient in a single state
		// to avoid duplications
		case ConnState_RegisteringNewClient:
			if connState.substate == ConnSubstate_ReadingClientName {
				if bytesRead > clientNameMaxLen {
					msg := []byte(fmt.Sprintf("name cannot exceed %d characters", clientNameMaxLen))
					s.sessionMessagesCh <- NewSessionMsg(newClient, msg)
					s.sessionMessagesCh <- NewSessionMsg(newClient, clientNameMsg)
				} else {
					if s.backend.HasClient(string(connState.input)) {
						msg := []byte(fmt.Sprintf("Name [%s] is occupied, try a different one", connState.input))
						s.sessionMessagesCh <- NewSessionMsg(newClient, msg)
						s.sessionMessagesCh <- NewSessionMsg(newClient, clientNameMsg)
					} else {
						connState.state = ConnState_AuthenticatingClient
						connState.substate = ConnSubstate_ReadingClientPassword
						connState.clientName = string(connState.input)
						s.sessionMessagesCh <- NewSessionMsg(newClient, clientPasswordMsg)
					}
				}
			} else if connState.substate == ConnSubstate_ReadingClientPassword {
				if bytesRead > clientPasswordMaxLen {
					msg := []byte(fmt.Sprintf("password cannot exceed [%d] characters", clientPasswordMaxLen))
					s.sessionMessagesCh <- NewSessionMsg(newClient, msg)
					s.sessionMessagesCh <- NewSessionMsg(newClient, clientPasswordMsg)
				} else {
					connState.substate = ConnSubstate_None
					connState.state = ConnState_ProcessingClientMessages
					connState.clientPassword = string(connState.input)

					s.backend.RegisterNewClient(
						connState.clientName,
						newClient.ipAddr,
						stateStr(ClientState_Connected),
						common.Sha256([]byte(connState.input)),
						newClient.joinTime,
					)

					// TODO: Handle a case when a client is connected from a different machine
					// Check whether a client with this name already exists and is disconnected.
					// Update its connection so we can read/write from it and change the ip address.
					if s.clients.existsWithState(connState.clientName, ClientState_Disconnected) {
						s.clients.removeClient(connState.clientName)
					}

					// client, exists := s.clientsMap[connState.clientName]
					// if exists {
					// 	if client.State == ClientState_Disconnected {
					// 		delete(s.clientsMap, newClient.ipAddr)
					// 	} else {
					// 		panic("client is connected from a different machine")
					// 	}
					// }
					if s.clients.existsWithState(newClient.ipAddr, ClientState_Pending) {
						s.clients.removeClient(newClient.ipAddr)
						newClient.name = connState.clientName
						newClient.state = ClientState_Connected
						s.clients.addClient(newClient.name, newClient) // add the same client, but by name
					} else {
						panic("client has to be present in a map")
					}

					s.onlineClientsCount.Add(1)

					// client, exists = s.clientsMap[newClient.ipAddr]
					// if exists && client.State == ClientState_Pending {
					// 	// Rehash the client based on its name rather than ip address
					// 	newClient.Name = connState.clientName
					// 	newClient.State = ClientState_Connected
					// 	delete(s.clientsMap, newClient.ipAddr)
					// 	s.clientsMap[newClient.Name] = newClient
					// } else {
					// 	panic("client has to be present in a map")
					// }
					// mu.Unlock()

					// TODO: This has to be renamed to connectedClientsCount (maybe we can only use one variable)
					// s.connectedClientCount.Add(1)

					// NOTE: This will force all test to fail.
					// For greeting messages we can rely on client's name,
					// because it happens after the client has been connected.
					// Maybe we should print another menu here which allows the client to choose whether
					// to display a chat history or to list all participants
					if s.onlineClientsCount.Load() > 1 {
						msg := []byte(fmt.Sprintf("client: [%s] joined", connState.clientName))
						s.greetingMessagesCh <- NewGreetingMsg(newClient, msg)
						// s.listClients(newClient)
						// if s.chatHistory.size() != 0 {
						// 	history := make([]string, s.chatHistory.size())
						// 	msgs := make([]ClientMessage, s.chatHistory.size())
						// 	s.chatHistory.flush(msgs)
						// 	for _, msg := range msgs {
						// 		m, _ := msg.Format()
						// 		history = append(history, string(m))
						// 	}
						// 	s.sessionMessagesCh <- NewSessionMsg(newClient, []byte(strings.Join(history, "\n")))
						// }
					}

					log.Info().Msgf("new client [%s] was registered", connState.clientName)

					// TODO: This function should accept a client
					go s.disconnectIfClientWasIdleForTooLong(conn, onIdleClientDisconnectedCh, abortDisconnectingClientCh, onClientErrorCh)
				}
			}

		case ConnState_AuthenticatingClient:
			if connState.substate == ConnSubstate_ReadingClientName {
				if bytesRead > clientNameMaxLen {
					msg := []byte(fmt.Sprintf("name cannot exceed %d characters", clientNameMaxLen))
					s.sessionMessagesCh <- NewSessionMsg(newClient, msg)
					s.sessionMessagesCh <- NewSessionMsg(newClient, clientNameMsg)
				} else {
					if !s.backend.HasClient(string(connState.input)) {
						msg := []byte(fmt.Sprintf("client with name [%s] doesn't exist", string(connState.input)))
						s.sessionMessagesCh <- NewSessionMsg(newClient, msg)
						s.sessionMessagesCh <- NewSessionMsg(newClient, clientNameMsg)
					} else {
						connState.state = ConnState_AuthenticatingClient
						connState.substate = ConnSubstate_ReadingClientPassword
						connState.clientName = string(connState.input)
					}
				}
			} else if connState.substate == ConnSubstate_ReadingClientPassword {
				if bytesRead > clientPasswordMaxLen {
					msg := []byte(fmt.Sprintf("password cannot exceed %d characters", clientPasswordMaxLen))
					s.sessionMessagesCh <- NewSessionMsg(newClient, msg)
					s.sessionMessagesCh <- NewSessionMsg(newClient, clientPasswordMsg)
				} else {
					connState.clientPassword = string(connState.input)

					if !s.backend.MatchClientPassword(connState.clientName, connState.clientPassword) {
						msg := []byte("password isn't correct, try again")
						s.sessionMessagesCh <- NewSessionMsg(newClient, msg)
						s.sessionMessagesCh <- NewSessionMsg(newClient, clientPasswordMsg)
					} else {
						connState.substate = ConnSubstate_None
						connState.state = ConnState_ProcessingClientMessages
						connState.clientPassword = string(connState.input)

						// mu.Lock()
						if s.clients.existsWithState(connState.clientName, ClientState_Disconnected) {
							s.clients.removeClient(connState.clientName)
						}

						// client, exists := s.clientsMap[connState.clientName]
						// if exists && client.State == ClientState_Disconnected {
						// 	delete(s.clientsMap, connState.clientName)
						// } else {
						// 	panic("client connected from a different machine")
						// }

						// NOTE: What if in between those calls the client was removed?
						// TODO: Write a function which does the swap operation.
						// Replaces an old client with a new client but with updated data.
						if s.clients.existsWithState(newClient.ipAddr, ClientState_Pending) {
							s.clients.removeClient(newClient.ipAddr)
							newClient.state = ClientState_Connected
							newClient.name = connState.clientName
							s.clients.addClient(newClient.name, newClient)
						} else {
							panic("client has to exist with a Pending status")
						}

						// client, exists = s.clientsMap[newClient.ipAddr]
						// if exists && client.State == ClientState_Pending {
						// 	// Rehash new client
						// 	newClient.State = ClientState_Connected
						// 	newClient.Name = connState.clientName
						// 	delete(s.clientsMap, newClient.ipAddr)
						// 	s.clientsMap[newClient.Name] = newClient
						// } else {
						// 	panic("client has to exist with a Pending status")
						// }
						// mu.Unlock()
						// s.connectedClientCount.Add(1)

						// if s.connectedClientCount.Load() > 1 {
						// 	msg := []byte(fmt.Sprintf("client: [%s] joined", connState.clientName))
						// 	s.greetingMessagesCh <- NewGreetingMsg(newClient, msg)
						// 	// listClientsAndChatHistory()
						// }

						log.Info().Msgf("client [%s] was authenticated", connState.clientName)

						// TODO: Rename to monitorIdleClient()
						go s.disconnectIfClientWasIdleForTooLong(conn, onIdleClientDisconnectedCh, abortDisconnectingClientCh, onClientErrorCh)
					}
				}
			}

		case ConnState_ProcessingClientMessages:
			s.clientMessagesCh <- NewClientMsg(newClient.ipAddr, newClient.name, buffer[:bytesRead])

			// NOTE: Clients are disconnected if they were idle for some duration.
			// Idle implies not sending any messages for some period of time.
			// If we receive at least one message, we should abort, since client is alive.
			abortDisconnectingClientCh <- struct{}{}

		case ConnState_CreatingChannel:
			panic("Creating channels is not supported!")
		}
	}

	if s.loggingEnabled {
		log.Info().Msgf("closing the connection with a client %s", newClient.name)
	}
	// BUG: When one client disconnects, session automatically closes the connection with other clients.

	// NOTE: We don't handle disconnected clients properly.
	// TODO: We don't receive any notifications about client leaving the session
	// mu.Lock()
	// State can only be changed inside this function, so we don't have to protect it with a mutex
	if s.clients.existsWithState(newClient.name, ClientState_Connected) { // newClient.State == Connected ?
		s.clients.updateState(newClient.name, ClientState_Disconnected)
	}

	// NOTE: The connection was aborted, but the client was inserted into a map with a Pending state.
	// We have to delete it manually, since it wasn't initialized properly.
	// Worth noting that in this case the key would be its Ip address instead of a name.
	if s.clients.existsWithState(newClient.ipAddr, ClientState_Pending) {
		s.clients.removeClient(newClient.ipAddr)
	}

	if s.connectionsCount.Load() > 1 {
		msg := []byte(fmt.Sprintf("client: [%s] left", newClient.name))
		s.greetingMessagesCh <- NewGreetingMsg(newClient, msg)
	}

	s.connectionsCount.Add(-1)

	if s.connectionsCount.Load() == 0 {
		s.timer.Reset(s.duration)
	}
}

func (s *Session) processMessages() {
	s.running = true

	for s.running {
		select {
		case msg := <-s.clientMessagesCh:
			if s.loggingEnabled {
				log.Info().Msgf("received client's message: %s", string(msg.Contents))
			}

			s.chatHistory.addMessage(msg)
			s.stats.MessagesReceived.Add(1)

			dropped, sent := s.clients.broadcastMessage(msg, []string{msg.SenderName}, []string{})
			_ = dropped
			_ = sent

			// TODO: Update stats about dropped messages and sent messages

			// for _, client := range s.clientsMap {
			// 	if client.State == ClientState_Connected {
			// 		// TODO: Is it possible to avoid string comparison for each client?
			// 		// We perform the comparison in order not to send a message to excluded client.
			// 		if strings.Compare(client.Name, msg.SenderName) != 0 {
			// 			fMsg, fMsgSize := msg.Format()
			// 			bWritten, err := writeBytesToClient(client, fMsg, fMsgSize)

			// 			// Client might have already disconnected by the time we are ready to write to it.
			// 			if err != nil || bWritten != fMsgSize {
			// 				log.Error().Msgf("failed to write bytes to [%s], error %s", client.Name, err.Error())
			// 			} else {
			// 				s.stats.MessagesSent.Add(1)
			// 			}
			// 		}
			//  }
			// }

		case msg := <-s.sessionMessagesCh:
			if s.loggingEnabled {
				log.Info().Msgf("received session's message: %s", string(msg.Contents))
			}

			// The recipient's state can be either ClientState_Pending or ClientState_Connected
			// s.mu.Lock()
			// mu := sync.Mutex{}
			// mu.Lock()
			// NOTE: These types of messages should only contain recipient's IP address
			// Or maybe not always(
			dropped, sent := s.clients.broadcastMessage(msg, []string{}, []string{msg.Recipient.ipAddr})
			_ = dropped
			_ = sent

			// fMsg, fMsgSize := msg.Format()
			// bWritten, err := writeBytesToClient(msg.Recipient, fMsg, fMsgSize)

			// if err != nil || bWritten != fMsgSize {
			// 	log.Error().Msgf("failed to write bytes to [%s], error %s", msg.Recipient.ipAddr, err.Error())
			// } else {
			// 	s.stats.MessagesSent.Add(1)
			// }
			// mu.Unlock()

		case msg := <-s.greetingMessagesCh:
			// Greeting messages should be broadcasted to all the clients, except sender,
			// which is specified inside exclude list.
			if s.loggingEnabled {
				log.Info().Msgf("received greeting message: %s", string(msg.Contents))
			}

			// TODO: For large amount of clients this has to be done in parallel,
			// because currently the running cost is O(n).
			// We can split the map into different subchunks and process them in a separate go routines.
			// mu := sync.Mutex{}
			// mu.Lock()
			dropped, sent := s.clients.broadcastMessage(msg, []string{msg.Exclude.name}, []string{})
			_ = dropped
			_ = sent

			// for _, client := range s.clientsMap {
			// 	if client.State == ClientState_Connected {
			// 		if strings.Compare(client.Name, msg.Exclude.Name) != 0 {
			// 			fMsg, fMsgSize := msg.Format()
			// 			bWritten, err := writeBytesToClient(client, fMsg, fMsgSize)

			// 			if err != nil || bWritten != fMsgSize {
			// 				log.Error().Msgf("failed to write bytes to [%s], error %s", client.Name, err.Error())
			// 			} else {
			// 				s.stats.MessagesSent.Add(1)
			// 			}
			// 		}
			// 	}
			// }
			// mu.Unlock()

		case <-s.timer.C:
			if s.loggingEnabled {
				log.Info().Msg("session timeout, no clients joined.")
			}
			s.running = false
			close(s.timeoutCh)
		}
	}
}

// NOTE: My impression that it would be better to introduce inboxes and outboxes for the session
// as well, so we can better track down send/received messages (in one place),
// instead of calling conn.Write function everywhere in the code and trying to handle if it fails.
// But we would have to define a notion how do distinguish between different message types.
// Maybe we have protect the whole struct with a mutex. What if the size changes?
func (s *Session) listClients(recipient *Client) {
	var clients []string
	var result string

	for k, v := range s.clients.getClients() {
		clients = append(clients, fmt.Sprintf("\t[%s] *%s", k, v))
	}
	result = strings.Join(clients, "\n")
	s.sessionMessagesCh <- NewSessionMsg(recipient, []byte("clients:\n"+result), true)
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
