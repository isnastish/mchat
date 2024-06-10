## Multiclient chat
This is a cli chat application written completely in Golang. Participants have common functionality, communication with each other via the network, an ability to create channels for sharing messages and files. The application supports multiple backends for storing the data (redis, dynamodb and in-memory for local development). Mode detailed explanation is provided in the architecture [architecture](architecture.md) document. Keep in mind that the project is still in development and requires more work in order to be considered as done.

## Running the application
There are two options to run the chat. You can either build both, a client and a session, and run them in  a separate terminals, or (the recommended way) run the session inside a docker container but build the client locally and then connect to the session with multiple clients. First, you need to build a docker image using the following command `docker build -t chat:latest`. Once the image is built, run the docker container. You can either start it in detached mode, with `-d` option, but in that case you won't see the logs. I suggest starting it with `docker run --rm -p 8080:8080 --name="chat-mock" chat:latest` command. 
Once the docker container is started, you will see the logs, if you don't, probably you are doing something wrong.
```log
{"level":"info","timestamp":"Monday, 10-Jun-24 05:48:25 UTC","message":"Running  memory backend"}
{"level":"info","timestamp":"Monday, 10-Jun-24 05:48:25 UTC","message":"Listening: [::]:8080"}
```
This indicates that the server is listening on port `8080` and ready to accept incoming connections.
Build a client in a separate terminal window with `go build -o ./bin/client/ github.com/isnastish/chat/services/client`. That will output client's binaries into `/bin/client/` directory. By running the client you should see the following menu:
```log
8:06AM INF Connected to 127.0.0.1:8080
options:
        {1}: register
        {2}: log in
        {3}: create channel
        {4}: select channels
        {5}: list members
        {6}: display chat history
        {7}: exit

        enter option:
```
Congratulations! You were able to successfully build and launch the application.