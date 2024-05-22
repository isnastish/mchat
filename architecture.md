## Backend storage

The backend interface should provide a functionality for persisting client's messages, as well as channels and their chat history. Three different backends were chosen. [in-memory](/architecture.md#in-memory) - for local development only. [redis](/architecture.md#redis) and [dynamodb](/architecture.md#dynamodb). It provides procedures for registering new clients, authenticating clients that were already registered, registering new channels and an ability to store chat history in each channel.  

```go

type Backend interface {
	// Check whether a participant with a given name exists.
	// Returns true if does, false otherwise.
	HasParticipant(username string) bool

	// Register a new participant with a unique username.
	// password is hashed using sha256 algorithm.
	ReigsterParticipant(username string, passwordShaw256 string, emailAddress string) bool

	// Authenticate an already registered participant.
	// If username or password is incorrect, returns false.
	AuthenticateParticipant(username string, passwordSha256 string) bool
	
	// Store a message in a backend storage.
	// If a message has a channel that it belongs to, it won't be displayed in a general chat history,
	// only in channel's history.
	StoreMessage(senderName string, sentTime string, message []byte, channelName string)

	// Register a new channel where clients can exchange messages.
	// All messages exchanged in a particular channel will be visible there only, 
	// and not in a general chat.
	// Each channel has an owner, who reservs the right to give a permission for other participants to join.
	// *In order to acquire a permission, it shold be requested.
	RegisterChannel(channelName string, ownerName string) bool
	
	// Returns history of all the messages sent in a general chat. 
	GetChatHistory() []*ClientMessage

	// Returns history of all the messages sent in a particular channel
	GetChannelHistory(channelName string) []*ClientMessage 
}
```

### In memory

### Redis

### DynamoDB

## Participatns

## Channels
Each participant should have an ability to create channels. The one who created a channel is considered to be its owner. He reserves the rights  

## Session
Session should maintain a list of active connections, since we cannot store the connection inside a database, and even if we could, I will change after the first reconnection. All information about participants should be stored in a backend storage, but in order to broadcast messages the session should hold all participants which are currently connected. Getting the list of all participants would require making a request to a backend storage, since not only currently connected participants should be displayed. If a participant disconnects, it should be deleted from a map of active connections, so we don't broadcast any messages to it. When a partcipant reconnects, the one should be inserted into a map. 

A `Participant` class should move into backend package together with messages, so we can reuse the code. Though it might be hard to decouple it later if we decide to implement a backend storage as a standalone service.

```go

type ConnectionState int32

const (
	Pending ConnectionState = iota + 1
	Connected
	// Disconnected (will be deleted from a map, so no need to maintain that state) 
)

type Connection struct {
	conn net.Conn
	ipAddr string
	state ConnectionState
	// A participant is retrieved from the backend storage if it
	// has existed before, or created. 
	participant *Participant
}

type ConnectionMap struct {
	connections map[string]*Connection
	mu          sync.Mutex
}

func (reader *ConnReader) disconnectIfIdle(duration time.Duration) {
	timeout := time.NewTimer(duration)
	for {
		select {
		case <-timeout.C:
			close(reader.IdleParticipantsCh)
			reader.Conn.Close()
			return
		case <-reader.AbortDisconnectingIdleParticipantCh:
			if !timeout.Stop() {
				<-timeout.C
			}
			timeout.Reset(duration)
		
		// In case we received an error inside handleConnection procedure, 
		// we should terminate this go routine so we don't have any leaks.
		case <-reader.ParticipantErrorCh:
			return
		}
	}
}

func (session *Session) handleConnection(conn net.Conn) {
	connection := Connection{ 
		state: Pending,
		conn: conn,
		ipAddr: conn.RemoteAddr().String(),
	}

	connReader := ConnReader{
		connection: connection
		state: DisplayingMenu,
		substate: Null,
		disconnectSignalCh: make(chan struct{})
		abortSignalCh: make(chan struct{})
	}

	go connReader.disconnectIfIdle()

	for !connReader.isState(Disconnecting) {
		data, dataLenght, err := connReader.Read()
		if err != nil && err != io.EOF {
			select {
			case <-connReader.onIdleClientDisconnectedCh:
				session.systemMessagesCh <- messages.NewSystemMessage(
					participant,
					[]byte("you were idle for too long, disconnecting"),
				)
				connReader.State = Disconnecting
			default:
				close(connReader.onClientErrorCh)
				connReader.State = Disconnecting
			}
		}

		if connReader.isState(ProcessingMenu) {

		} else if connReader.isState(AcceptingMessages) {
			if connReader.isSubstate(ProcessingInChannel) {
				// Send messages inside a channel.
			}
		} else if connReader.isState(RegisteringNewParticipant) {

		} else if connReader.isState(AuthenticatingNewParticipant) {

		} else if connReader.isState(CreatingNewChannel) {

		}
	}

	session.connetions.DeleteConnention(connection.ipAddr)
	conn.Close()
}

type Session struct {
	connections map[string]*Connection
}
```

```go
for {
	conn, err := session.listener.Accept()
	if err != nil {
		continue
	}
	go session.handleConnection(conn)
}
```