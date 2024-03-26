package main

// TODO: Implement retries, If client failed to connect to the session
// Set maximum retries limit.

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	_ "time"

	log "github.com/isnastish/chat/pkg/logger"
)

type ClientState struct {
	network string
	address string
	logger  log.Logger
}

var client ClientState

func recvMessages(serverConn net.Conn, done chan struct{}) {
	input := bufio.NewScanner(serverConn)
	for input.Scan() { // blocks
		text := input.Text()

		// TODO: Limit a user name to 64 characters.
		if strings.EqualFold(text, "username:") {
			client.logger.Info().Msg("reading username")

			reader := bufio.NewReader(os.Stdin)
			if username, err := reader.ReadString('\n'); err != nil {
				done <- struct{}{}
			} else {
				if _, err := io.WriteString(serverConn, fmt.Sprintf("username: %s", username)); err != nil {
					done <- struct{}{}
					return
				}
			}
		} else {
			io.WriteString(os.Stdout, text)
		}
	}

	done <- struct{}{}

	// if _, err := io.Copy(os.Stdout, serverConn); err != nil {
	// 	done <- struct{}{}
	// }
}

func sendMessages(serverConn net.Conn, done chan struct{}) {
	// for {
	// 	if _, err := io.WriteString(serverConn, fmt.Sprintf("addr[%s]: hello\n", serverConn.LocalAddr().String())); err != nil {
	// 		done <- struct{}{}
	// 	}
	// 	time.Sleep(3000 * time.Millisecond)
	// }
}

func main() {
	client = ClientState{
		logger: log.NewLogger("debug"),
	}

	flag.StringVar(&client.network, "network", "tcp", "Network protocol [TCP|UDP]")
	flag.StringVar(&client.address, "address", "127.0.0.1:8080", "Address, for example: localhost:8080")

	flag.Parse()

	serverConn, err := net.Dial(client.network, client.address)
	if err != nil {
		client.logger.Error().Msgf("failed to connected to the server: %v", err)
	}

	client.logger.Info().Str("Server address", serverConn.RemoteAddr().String()).Msg("connected to the server")

	done := make(chan struct{})
	go recvMessages(serverConn, done)
	go sendMessages(serverConn, done)
	<-done

	serverConn.Close()
}
