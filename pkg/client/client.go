package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/reader"
	"github.com/isnastish/chat/pkg/types"
	"github.com/isnastish/chat/pkg/utilities"
)

type Config struct {
	Network      string
	Addr         string
	RetriesCount int
}

type client struct {
	config          *Config
	sessionConn     net.Conn
	quitChan        chan struct{}
	inMessagesChan  chan *types.ChatMessage
	outMessagesChan chan *types.ChatMessage
	running         bool
}

func CreateClient(config *Config) *client {
	client := &client{
		config:          config,
		quitChan:        make(chan struct{}),
		inMessagesChan:  make(chan *types.ChatMessage),
		outMessagesChan: make(chan *types.ChatMessage),
	}

	return client
}

func tryConnect(network, address string, ctx context.Context, retriesCount int, delay time.Duration) net.Conn {
	for retries := 0; ; retries++ {
		sessionConn, err := net.Dial(network, address)
		if err == nil {
			return sessionConn
		}

		if retries >= retriesCount {
			return nil
		}

		log.Logger.Info("Attemp %d to connect failed, retrying in %vs", delay)

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil
		}
	}
}

func (c *client) Run() {
	// if conn := tryConnect(c.config.Network, c.config.Addr, context.Background());

	defer c.sessionConn.Close()

	c.running = true

	go c.recv()
	go c.send()

	for c.running {
		select {
		case msg := <-c.inMessagesChan:
			log.Logger.Info("Message received")
			fmt.Printf("%s", msg.Contents.String())

		case msg := <-c.outMessagesChan:
			utilities.WriteBytes(c.remoteConn, msg.Contents)

		case <-c.quitChan:
			c.running = false
		}
	}

}

func (c *client) recv() {
	reader := reader.NewReader(c.sessionConn)

	for {
		err := reader.Read()
		if err != nil && err != io.EOF {
			log.Logger.Error("failed to read from a remote connnection: %s", err.Error())
			c.remoteConn.Close()
			close(c.quitCh)
			break
		}

		if nbytes == 0 {
			log.Logger.Error("remote session closed the connection")
			close(c.quitCh)
			return
		}
		c.incommingCh <- Message{data: buf[:nbytes]}
	}
}

func (c *client) send() {
	reader := reader.NewReader(os.Stdin)

	for {
		err := reader.Read()
		if err != nil && err != io.EOF {
			log.Logger.Error("failed to read the input")
			break
		}

		c.outMessagesChan <- &types.ChatMessage{
			data: []byte(strings.Trim(string(buf[:bytesRead]), " \r\n\t\f\v"))
		}
	}
}
