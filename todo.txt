TODO: 
[x] Displaying participants should be done in a different way.
    We shouldn't display the time, only the connected peers.
    Time should only be used for messages.
    Example: 
    participants:
        [alexey] *connected
        [misha]  *connected
        [polina] *disconnected

[x] Be able to switch between different storages, mysql vs redis. 
    This requires implementing a support for Redis first.
    Implement a backend which can switch between sql and in-memory storage.
    Create a pkg/backend folder with implementations for both backends.
    Use interface{} for the backend.

[x] Backend should be determined based on env variable DATABASE_BACKEND, 
    and can be either "redis", "mysql" or "memory".

[x] Create an instruction how to start redis-server on windows:
    1. Enter wsl: 
    2. curl -fsSL https://packages.redis.io/gpg | sudo gpg --dearmor -o /usr/share/keyrings/redis-archive-keyring.gpg
    3. echo "deb [signed-by=/usr/share/keyrings/redis-archive-keyring.gpg] https://packages.redis.io/deb $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/redis.list
    4. sudo apt-get update
    5. sudo apt-get install redis
    6. sudo service redis-server start
    7. redis-cli

[x] When the server closes a connection, the client hangs in one of the go routines.
    Make it shutd down gracefully.

[x] Include client's name when sending a message about newly arrived clients. (on Joined)
    The message should include time, when a participant has joined, and it's provided name.
    [12:00:00] @participantName Joined

[ ] Query the database for a list of available participants.

[ ] Secure the connection with TLS.

[ ] Connect to Redis with TLS (only for production, not for local development)

[ ] Allow clients to host their own sessions. A new session should executed in a separate go routine
    or even in a separate process. And we probably would have to maintain a map of sessions somewhere.
    Go:
    var sessionMap map[string]*Session
    Once a session has been created, we have to add it to a sessions map.

[ ] Enable tracing and metrics for redis backend implementation.

[ ] Introduce a memory backend. (most likely in a form of a map[string]*map[string]string), 
    So we can switch between different backends, Redis, Mysql and in memory backend.

[ ] Authenticate all clients joining the session.

[ ] All the data from sessions should be dumped into a database (either mysql or redis) at the end.
[ ] Output the list of sessions (channels).
    Sessions: 
    [*] <name> [number of participants], [creation_data], [host - a client who hosted a session]

    Ex:
    [*] @my_session participants: [32], created: [23-02-2024, 11:03:00], host: @some_user_who_hosted_the_session

[ ] Replace build.bat with build.py file, so we can run on all platforms.

[ ] We shouldn't use logs while displaying the messages received by the clients.
    Logs should only be available on the server side.

[ ] Rename client's.addr field to name, when the functionality for processing client's names is implemented.

[ ] Rename `common` package to `utilities`.

[ ] Encode the data on the sender side using base64 encoding, and decode the data on the receiver side.

[ ] Right now we have three types of messages, which develops into maintaining three types of channels.
    Since we now know all the edge cases and difference between message types, pull everything into a single 
    struct for clarity. Maybe introduce an enum with corresponding messages types.

[ ] Adjust metrics according global refactoring.

[ ] When the client starts first, then the server, we get an error, null-ptr dereference.
    Make sure that when the client starts first, and then the server, the client will be connected after a couple of retries.

[ ] Improve the client in general. Go through the code and verify that everything is consistent.

[x] Prompt for client's password.

[x] How do we distinguish between already registered clients and those who came recently?
    We should provide some options for the clients, for example, on joining we might 
    display a menu:

    register:          [1]
    log in:            [2]
    list participants: [3] (maybe we cannot see participants list, until we were authenticated)

    So then, while doing the authentication, we can distinguish whether the name specified already exists 
    and we just have to match it against the existing one or we have to registerd the client.
    Only in a case of registering a new client we have to check whether a name is already exists, 
    and print a user error.

    *Menu can be displayed on the client side instead, and the server should only receive the corresponding
    data. As an exercise, let's do it on the server side first.

[x] Implemented the neccessary functionality to Register/Login/List participants/Exit.

[x] Restructure the code so that conn.Read() happens only in one place.
    Finilize the connection reader

[ ] Send notifications on email about send/received messages and about messages of other participants.