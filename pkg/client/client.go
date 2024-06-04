package client

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/types"
	"github.com/isnastish/chat/pkg/utilities"
)

type MenuOptionType int8

const (
	RegisterParticipant     MenuOptionType = 0x01
	AuthenticateParticipant MenuOptionType = 0x02
	CreateChannel           MenuOptionType = 0x03
	SelectChannel           MenuOptionType = 0x04
	Exit                    MenuOptionType = 0x05
)

var menuOptionsTable = []string{
	"Register",          // Register a new participant.
	"Log in",            // Authenticate an already registered participant.
	"Create channel",    // Create a new channel.
	"Select channels",   // Select a channel for writing messages.
	"List participants", // List all participants
	"Exit",              // Exit the sesssion.
}

var menuMessageHeader = []byte("options:\r\n")
var channelsMessageHeader = []byte("channels:\r\n")
var participantListMessageHeader = []byte("participants:\r\n")
var usernameMessageContents = []byte("username: ")
var passwordMessageContents = []byte("password: ")
var emailAddressMessageContents = []byte("email address: ")
var channelsNameMessageContents = []byte("channel's name: ")
var channelsDescMessageContents = []byte("channel's desc: ")

var passwordValidationFailedMessageContents = []byte("password validation failed")
var emailAddressValidationFailedMessageContents = []byte("email address validation failed")
var usernameValidationFailedMessageContents = []byte("username validation failed")

const retriesCount int32 = 5

type ClientConfig struct {
	Network      string
	Addr         string
	Port         int
	RetriesCount int
}

type client struct {
	config          *ClientConfig
	remoteConn      net.Conn
	quitChan        chan struct{}
	inMessagesChan  chan *types.ChatMessage
	outMessagesChan chan *types.ChatMessage
}

func CreateClient(network, address string) (*client, error) {
	var remoteConn net.Conn
	var nretries int32
	var lastErr error

	for nretries < retriesCount {
		remoteConn, lastErr = net.Dial(network, address)
		if lastErr != nil {
			nretries++
			log.Logger.Warn("connection failed, retrying...")
			time.Sleep(3000 * time.Millisecond)
		} else {
			log.Logger.Info("connected to remote session: %s", remoteConn.RemoteAddr().String())
			break
		}
	}

	if nretries != 0 {
		return nil, lastErr
	}

	c := &client{
		network:     network,
		address:     address,
		remoteConn:  remoteConn,
		quitCh:      make(chan struct{}),
		incommingCh: make(chan Message),
		outgoingCh:  make(chan Message),
	}

	return c, nil
}

func (c *client) Run() {
	go c.recv()
	go c.send()

Loop:
	for {
		select {
		case msg := <-c.inMessagesChan:
			log.Logger.Info("Message received")
			fmt.Printf("%s", msg.Contents.String())

		case msg := <-c.outMessagesChan:
			utilities.WriteBytes(c.remoteConn, msg.Contents)

		case <-c.quitChan:
			break Loop
		}
	}
	c.remoteConn.Close()
}

func (c *client) recv() {
	for {
		buf := make([]byte, 4096)
		nbytes, err := c.remoteConn.Read(buf)
		if err != nil && err != io.EOF {
			log.Logger.Error("failed to read from the remote connnection: %s", err.Error())
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
	buf := make([]byte, 4096)
	inputReader := bufio.NewReader(os.Stdin)

	for {
		bytesRead, err := inputReader.Read(buf)
		if err != nil && err != io.EOF {
			log.Logger.Error("failed to read the input")
			break
		}

		c.outgoingCh <- Message{data: []byte(strings.Trim(string(buf[:bytesRead]), " \r\n\t\f\v"))}
	}
}
