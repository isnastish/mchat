// TODO: Introduce a limit on the input in a chat. Meaning that the client won't
// be able to write more then, let's say, 1024 characters in a single messages,
// because they simply won't be sent to the server.
package client

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/types"
	"github.com/isnastish/chat/pkg/utilities"
)

type Config struct {
	Network      string
	Addr         string
	RetriesCount int
	TLSConfig    *tls.Config
}

type client struct {
	config           *Config
	remoteConn       net.Conn
	quitChan         chan struct{}
	incomingMessages chan *types.ChatMessage
	outgoingMessages chan *types.ChatMessage
	ctx              context.Context
	cancel           context.CancelFunc
}

func CreateClient(config *Config) *client {
	ctx, cancle := context.WithCancel(context.Background())
	return &client{
		config:           config,
		quitChan:         make(chan struct{}),
		incomingMessages: make(chan *types.ChatMessage),
		outgoingMessages: make(chan *types.ChatMessage),
		ctx:              ctx,
		cancel:           cancle,
	}
}

func (c *client) tryConnect(delay time.Duration) (net.Conn, bool) {
	for retries := 0; ; retries++ {
		sessionConn, err := tls.Dial(c.config.Network, c.config.Addr, c.config.TLSConfig)
		if err == nil {
			log.Logger.Info("Connected to %s", sessionConn.RemoteAddr().String())
			return sessionConn, true
		}
		if retries >= c.config.RetriesCount {
			return nil, false
		}

		log.Logger.Info("Attemp %d to connect failed, retrying in %v", retries, delay)

		<-time.After(delay)
	}
}

func (c *client) Run() {
	conn, succeeded := c.tryConnect(2 * time.Second)
	if !succeeded {
		log.Logger.Error("Failed to connect")
		return
	}
	c.remoteConn = conn

	defer c.remoteConn.Close()

	go c.handleRemoteConnection()
	go c.processInput()

	for {
		select {
		case msg := <-c.incomingMessages:
			fmt.Printf("%s", msg.Contents.String())

		case msg := <-c.outgoingMessages:
			util.WriteBytesToConn(c.remoteConn, msg.Contents.Bytes(), msg.Contents.Len())

		case <-c.ctx.Done():
			return
		}
	}
}

func (c *client) handleRemoteConnection() {
	for {
		tmpBuf := make([]byte, 1024)
		bytesRead, err := c.remoteConn.Read(tmpBuf)
		buffer := bytes.NewBuffer(tmpBuf[:bytesRead] /*util.TrimWhitespaces(tmpBuf[:bytesRead])*/)

		if err != nil && err != io.EOF {
			log.Logger.Error("Failed to read from a remote connection %v", err)
			c.cancel()
			return
		}

		if bytesRead == 0 { // io.EOF
			log.Logger.Error("Remote closed the connection")
			c.cancel()
			return
		}

		c.incomingMessages <- types.BuildChatMsg(buffer.Bytes(), "none")
	}
}

func (c *client) processInput() {
	reader := bufio.NewReader(os.Stdin)
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			tmpBuf := make([]byte, 1024)
			bytesRead, err := reader.Read(tmpBuf)
			buffer := bytes.NewBuffer(util.TrimWhitespaces(tmpBuf[:bytesRead]))

			// TODO: Try to recover somehow or close the remote connection?
			if err != nil && err != io.EOF {
				log.Logger.Error("Failed to read the input %v", err)
				return
			}

			c.outgoingMessages <- types.BuildChatMsg(buffer.Bytes(), "none")
		}
	}
}
