package main

import (
	"flag"
	"fmt"
	log "github.com/isnastish/chat/pkg/logger"
	"io"
	"net"
	"os"
	"time"
)

type ClientState struct {
	logger *log.Logger

	network string
	address string
}

var client ClientState

func recvMessages(serverConn net.Conn, done chan struct{}) {
	if _, err := io.Copy(os.Stdout, serverConn); err != nil {
		done <- struct{}{}
	}
}

func sendMessages(serverConn net.Conn, done chan struct{}) {
	for {
		if _, err := io.WriteString(serverConn, fmt.Sprintf("addr[%s]: hello\n", serverConn.LocalAddr().String())); err != nil {
			done <- struct{}{}
		}
		time.Sleep(3000 * time.Millisecond)
	}
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
