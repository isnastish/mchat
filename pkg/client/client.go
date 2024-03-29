package client

import (
	"bufio"
	"io"
	"net"
	"os"
	"time"

	lgr "github.com/isnastish/chat/pkg/logger"
	sts "github.com/isnastish/chat/pkg/stats"
)

const retriesCount int32 = 5

type Message struct {
	data []byte
}

type Client struct {
	remoteConn net.Conn
	network    string
	address    string
	// log        l.Logger

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
		// log:        log,
		network:    network,
		address:    address,
		remoteConn: remoteConn,

		quitCh: make(chan struct{}),

		// a message can be a struct with time and data ([]byte) members,
		// to signify when the messages arrived.

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
		case inMsg := <-c.incommingCh:
			c.stats.MessagesReceived.Add(1)

			log.Info().Msg("received message")
			{
				log.Info().Msg(string(inMsg.data))
			}

		case outMsg := <-c.outgoingCh:
			c.stats.MessagesSent.Add(1)

			log.Info().Msg("sending message")
			{
				nbytes, err := c.remoteConn.Write(outMsg.data)
				log.Info().Int("count", nbytes).Msg("wrote bytes")
				if err != nil {
					log.Error().Msgf("failed to write to remote connection: %s", err.Error())
					// return
				}
			}
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
		// If the client has been disconnected manually, we have to shutdown it completely
		nbytes, err := c.remoteConn.Read(buf)
		if err != nil && err != io.EOF {
			log.Error().Msgf("failed to read from the remote connnection: %s", err.Error())
			// should we shutdown this client or handle it more gracefully?
			// Since send() function is still running, we cannot cancel recv function.
			break
		}

		if nbytes == 0 {
			log.Info().Msg("remote session closed the connection")
			// force the send() goroutine to complete
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
		nbytes, err := inputReader.Read(buf)
		if err != nil && err != io.EOF {
			log.Error().Msg("failed to read the input")
			break
		}

		log.Info().Str("contents", string(buf[:nbytes])).Msg("input received")

		c.outgoingCh <- Message{data: buf[:nbytes]}
	}
}
