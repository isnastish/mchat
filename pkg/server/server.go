package main

import (
	"bufio"
	"fmt"
	log "github.com/isnastish/chat/pkg/logger"
	"io"
	"net"
	_ "os"
	"sync"
	"time"
)

var logger *log.Logger

func handleConnection(conn net.Conn, disconnected chan net.Conn, connections map[string]net.Conn) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	defer wg.Wait()

	// go routine is not necessary
	go func() {
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			text := scanner.Text()
			msg := fmt.Sprintf("%s\n", text)
			for _, nextConn := range connections {
				if nextConn != conn { // Don't send a message back to the same client.
					if _, err := io.WriteString(nextConn, msg); err != nil {
						disconnected <- conn
						return
					}
				}
			}
		}
		wg.Done()
	}()

	// for {
	// 	if _, err := io.WriteString(conn, time.Now().Format(time.RFC822)+"\n"); err != nil {
	// 		disconnected <- conn
	// 		return
	// 	}
	// 	time.Sleep(1000 * time.Millisecond)
	// }
}

func acceptIncomingConnections(listener net.Listener, connected, disconnected chan net.Conn, connections map[string]net.Conn) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Warn().Msgf("connection failed: %v", conn.RemoteAddr().String())
			continue
		}
		connected <- conn

		go handleConnection(conn, disconnected, connections)
	}
}

func main() {
	logger = log.NewLogger("debug")

	connections := map[string]net.Conn{}

	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		logger.Error().Msgf("failed to created a listener: %v", err.Error())
	}

	fmt.Printf("Listening: %s\n", listener.Addr().String())

	connected := make(chan net.Conn)
	disconnected := make(chan net.Conn) // only use the name

	go acceptIncomingConnections(listener, connected, disconnected, connections)

	timeout := 5000 * time.Millisecond
	timer := time.NewTimer(timeout)

Loop:
	for {
		select {
		case conn := <-connected:
			logger.Info().Str("addr", conn.RemoteAddr().String()).Msg("client connected")
			connections[conn.RemoteAddr().String()] = conn
			if !timer.Stop() { // drain channel
				<-timer.C
			}
		case conn := <-disconnected:
			conn.Close()
			logger.Info().Str("addr", conn.RemoteAddr().String()).Msg("client disconnected")
			delete(connections, conn.RemoteAddr().String())
			if len(connections) == 0 {
				timer.Reset(timeout)
			}

		case <-timer.C:
			logger.Info().Msg("no connections discovered, exiting")
			break Loop
		}
	}
}
