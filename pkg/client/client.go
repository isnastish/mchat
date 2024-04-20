package client

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/isnastish/chat/pkg/common"
	lgr "github.com/isnastish/chat/pkg/logger"
	sts "github.com/isnastish/chat/pkg/stats"
)

const retriesCount int32 = 5

// Pull out in a separate package shared by both, client and a server?
type Message struct {
	data []byte
}

type Client struct {
	remoteConn net.Conn
	network    string
	address    string

	quitCh chan struct{}

	incommingCh chan Message
	outgoingCh  chan Message

	stats sts.Stats
}

var log = lgr.NewLogger("debug")

func NewClient(network, address string) (*Client, error) {
	var remoteConn net.Conn
	var nretries int32
	var lastErr error

	for nretries < retriesCount {
		remoteConn, lastErr = net.Dial(network, address)
		if lastErr != nil {
			nretries++
			log.Info().Msg("connection failed, retrying...")
			time.Sleep(3000 * time.Millisecond)
		} else {
			log.Info().Msgf("connected to remote session: %s", remoteConn.RemoteAddr().String())
			break
		}
	}

	if nretries != 0 {
		return nil, lastErr
	}

	c := Client{
		network:    network,
		address:    address,
		remoteConn: remoteConn,

		quitCh: make(chan struct{}),

		incommingCh: make(chan Message),
		outgoingCh:  make(chan Message),
	}

	return &c, nil
}

func (c *Client) Run() {
	go c.recv()
	go c.send()

Loop:
	for {
		select {
		case msg := <-c.incommingCh:
			c.stats.MessagesReceived.Add(1)
			msgStr := string(msg.data)
			// TODO:
			// Introduce a table of reserved commands. All commans should be prefixed with @ to distinguish between regular messages.
			// @name, @new_channel, @list_participants ...
			if strings.Contains(msgStr, "@name:") {
				fmt.Printf("%s", msgStr)
			} else if strings.Contains(msgStr, "@password:") {
				fmt.Printf("%s", msgStr)
			} else {
				fmt.Printf("%s\n", msgStr)
			}

		case msg := <-c.outgoingCh:
			messageSize := len(msg.data)

			var bytesWritten int
			for bytesWritten < messageSize {
				n, err := c.remoteConn.Write(msg.data[bytesWritten:])
				if err != nil {
					log.Error().Msgf("failed to write to a remote connection: %s", err.Error())
					break
				}
				bytesWritten += n
			}
			if bytesWritten == messageSize {
				c.stats.MessagesSent.Add(1)
			} else {
				c.stats.MessagesDropped.Add(1)
			}
			// log.Info().Msgf("sent message: %s", string(msg.data))

		case <-c.quitCh:
			break Loop
		}
	}

	c.remoteConn.Close()
	sts.DisplayStats(&c.stats, sts.Client)
}

func (c *Client) recv() {
	buf := make([]byte, 4096)

	for {
		nbytes, err := c.remoteConn.Read(buf)
		if err != nil && err != io.EOF {
			log.Error().Msgf("failed to read from the remote connnection: %s", err.Error())
			c.remoteConn.Close()
			close(c.quitCh)
			break
		}

		if nbytes == 0 {
			log.Info().Msg("remote session closed the connection")
			close(c.quitCh)
			return
		}

		// log.Info().Msgf("%s", string(buf[:nBytes]))
		// c.incommingCh <- buf[:nBytes]

		c.incommingCh <- Message{data: buf[:nbytes]}
	}
}

func (c *Client) send() {
	buf := make([]byte, 4096)
	inputReader := bufio.NewReader(os.Stdin)

	for {
		bytesRead, err := inputReader.Read(buf)
		if err != nil && err != io.EOF {
			log.Error().Msg("failed to read the input")
			break
		}

		c.outgoingCh <- Message{data: common.StripCR(buf, bytesRead)}
	}
}
