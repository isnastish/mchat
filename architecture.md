## Backend
The application supports multiple backends for persisting the chat history, channels and participant's data. Interaction with a storage is done through the backend package which hides all the implementation details. When deployed in the cloud, it uses [redis](architecture.md#redis) or [dynamodb](architecture.md#dynamodb), otherwise [in-memory](architecture.md#memory) storage is used for local development.
All backends implement the `Backend` interface and support the functionality for registering new participants, authenticating participants, storing chat's history as well as channel's history and an ability to query the storage for a particular information, get a message history for a specific period of time, etc. More features will be added in the future as the project develops.

### Redis

### Dynamodb

### Memory
Memory backend implements a `Backend` interface 

## Handling connections
Every new connection is processed in a separate goroutine. A connection is a one-to-one mapping to a participant. The session only maintains a map of active connections. When a new participant joins, an instance of a `Connection` struct is created and inserted into a connections map. The map itself is designed to be thread-safe. When a participant disconnects, a connection is removed from the map. The list of all participants (currently connected and disconnected) is stored in a remote database such as Redis on DynamoDB. The reason for maintaining a map of active connection is because we need somehow to send messages to them, and it is not possible to store a `net.Conn` struct in a database, and even if we could, it will be out of date once a participant disconnects. Thus, the data about all the participants is stored in a database, and only currently connected once are stored in a memory of a session to broadcast the messages. Thus the connection map grows and shrinks during the lifetime of a program. 

Reading bytes from a connection is done with the help of a `Reader` which operates as a state machine. It changes its state based on the bytes read from a connection. For example, if the current state is `AuthenticatingParticipant` the reader would assume that the first bytes read would correspond to the username and the second set of bytes read will correspond to the the password. Thus, with a help of a state machine we could have a `conn.Read` only in one place.

## Messages
There are two types of messages, system messages and participant's messages with `SystemMessage` and `ParticipantMessage` structs representing each type respectively. System messages are sent by the session itself rather than by participants. They are used to broadcast special messages like requesting for the username or a password, and reporting the errors.
On the other hand, participant messages are actual messages coming from participants itself and they are stored in a remote database (Redis or DynamoDB) and form a chat history.
Processing of all the messages is done inside `processMessages` routine with a help of a `select` statement, since messages are sent on different channels. System messages are sent via the `session.systemMessagesCh` channel and messages from participants are sent via `session.participantMessagesCh` channel.

## Disconnecting idle participants
If a participant was idle (didn't send any message) for specified time duration, it is disconnected with a corresponding notification.  