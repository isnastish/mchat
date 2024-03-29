package client

import (
	"bufio"
	l "github.com/isnastish/chat/pkg/logger"
	"io"
	"net"
	"os"
	_ "strings"
	"sync"
	"time"
)

// TODO: Implement inboxes and outboxes

const retriesCount int32 = 5

type Client struct {
	remoteConn net.Conn
	network    string
	address    string
	log        l.Logger

	inputCh chan []byte
	quitCh  chan struct{}
	wg      sync.WaitGroup
}

func NewClient(network, address string) (*Client, error) {
	log := l.NewLogger("debug")

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
		log:        log,
		network:    network,
		address:    address,
		remoteConn: remoteConn,

		inputCh: make(chan []byte),
		quitCh:  make(chan struct{}),

		wg: sync.WaitGroup{},
	}

	return &c, nil
}

func (c *Client) Run() {
	c.wg.Add(2)

	go c.recv()
	go c.send()

	c.wg.Wait()
	c.remoteConn.Close()
}

func (c *Client) recv() {
	defer c.wg.Done()

	buf := make([]byte, 4096)

	for {
		// If the client has been disconnected manually, we have to shutdown it completely
		nBytes, err := c.remoteConn.Read(buf)
		if err != nil && err != io.EOF {
			c.log.Error().Msgf("failed to read from the remote connnection: %s", err.Error())
			// should we shutdown this client or handle it more gracefully?
			// Since send() function is still running, we cannot cancel recv function.
			break
		}

		if nBytes == 0 {
			c.log.Info().Msg("remote session closed the connection")
			// force the send() goroutine to complete
			close(c.quitCh)
			return
		}

		c.log.Info().Msgf("%s", string(buf[:nBytes]))
	}
}

func (c *Client) readInput() {
	inputReader := bufio.NewReader(os.Stdin)
	buf := make([]byte, 4096)

	for {
		nbytes, err := inputReader.Read(buf)
		if err != nil && err != io.EOF {
			c.log.Error().Msg("failed to read the input")
			break
		}

		c.log.Info().Str("contents", string(buf[:nbytes])).Msg("input received")

		c.inputCh <- buf[:nbytes]
	}
}

func (c *Client) send() {
	defer c.wg.Done()
	go c.readInput()

Loop:
	for {
		select {
		case msg := <-c.inputCh:
			nBytes, err := c.remoteConn.Write(msg)
			c.log.Info().Int("count", nBytes).Msg("wrote bytes")
			if err != nil {
				c.log.Error().Msgf("failed to write to remote connection: %s", err.Error())
				return
			}
		case <-c.quitCh:
			break Loop
		}
	}
}
