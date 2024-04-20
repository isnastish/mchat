package session

import (
	"fmt"
	"io"
	"net"
	"strings"
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

// NOTE: If we failed to write to a connection, that's still alright.
// the most important thing is that we don't have concurrent reads/writes to/from clientsMap

// NOTE: Using a single mutex and locking it in multiple palces was a stupid idea,
// which caused a deadlock (obviously).

// fatal error: concurrent map read and map write
// 20 Apr 24 14:33 CEST |INFO| registering new client [Mark Lutz]
// fatal error: concurrent map iteration and map write

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

// TODO: More descriptive name
type ConnState struct {
	conn           net.Conn
	state          int
	substate       int
	clientName     string
	clientPassword string
	input          []byte
}

type Settings struct {
	NetworkProtocol string // tcp|udp
	Addr            string
	BackendType     int
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
	clientMessagesCh   chan *ClientMessage
	sessionMessagesCh  chan *SessionMessage
	greetingMessagesCh chan *GreetingMessage
	running            bool
	quitCh             chan struct{}
	stats              sts.Stats // atomic
	// TODO: We need to protect the storage with a mutex.
	// And we would have to protect clientsMap with a mutex as well.
	backend     bk.DatabaseBackend
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

var clientNameMsg = []byte("@name: ")
var clientPasswordMsg = []byte("@password: ")
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
		clients:            newMap(),
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

func (s *Session) handleConnection(conn net.Conn) {
	defer func() {
		// TODO: What happens if we try to close an already closed connection?
		conn.Close()
	}()

	// mu := sync.Mutex{}

	newClient := &Client{
		conn:     conn,
		ipAddr:   conn.RemoteAddr().String(),
		state:    ClientState_Pending,
		joinTime: time.Now(),
	}

	s.connectionsCount.Add(1)

	// mu.Lock()
	s.clients.addClient(newClient.ipAddr, newClient)
	// s.clientsMap[newClient.ipAddr] = newClient // This should be deleted if the client disconnects
	// s.stats.ConnCount++
	// mu.Unlock()

	s.timer.Stop()

	// TODO: Correctly update the stats

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

	connState := ConnState{
		conn:     conn,
		state:    ConnState_ProcessingMenu,
		substate: ConnSubstate_None,
	}

	onIdleClientDisconnectedCh := make(chan struct{})
	abortDisconnectingClientCh := make(chan struct{})
	onClientErrorCh := make(chan struct{}) // client error

	s.displayMenu(newClient)

	// listClientsAndChatHistory := func() {
	// 	contents := []byte(fmt.Sprintf("client: [%s] joined", thisClient.name))
	// 	exclude := thisClient.ipAddr
	// 	s.greetingMessagesCh <- NewGreetingMsg(contents, exclude)
	// 	s.listParticipants(conn, newClient.ipAddr)
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
			case <-onIdleClientDisconnectedCh:
				// Notify the client that it will be disconnected
				contents := []byte("you were idle for too long, disconnecting")
				s.sessionMessagesCh <- NewSessionMsg(newClient, contents)
				if s.loggingEnabled {
					log.Info().Msgf("client: [%s]: has been idle for too long, disconnecting", newClient.ipAddr)
				}
			default:
				if s.loggingEnabled {
					log.Error().Msgf("failed to read bytes from the client: %s", err.Error())
				}
				close(onClientErrorCh)
				break Loop
			}
			// return
		}

		// NOTE: This happens exaclty when the opposite side closes the connection.
		if bytesRead == 0 {
			if s.loggingEnabled {
				log.Info().Msg("opposite side closed the connection")
			}
			close(onClientErrorCh) // unbock disconnectIdleClient function, so we don't have any leaks
			break
		}

		connState.input = common.StripCR(buffer, bytesRead)

		switch connState.state {
		case ConnState_ProcessingMenu:
			switch string(connState.input) {
			case RegisterClientOption:
				// TODO: Collapse into a function
				connState.state = ConnState_RegisteringNewClient
				connState.substate = ConnSubstate_ReadingClientName
				s.sessionMessagesCh <- NewSessionMsg(newClient, clientNameMsg)

			case AuthenticateClientOption:
				connState.state = ConnState_AuthenticatingClient
				connState.substate = ConnSubstate_ReadingClientName
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
