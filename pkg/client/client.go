package client

// TODO: Add maximum retries limit.

import (
	"bufio"
	"io"
	"net"
	"os"
	"sync"

	l "github.com/isnastish/chat/pkg/logger"
)

type Client struct {
	remoteConn net.Conn
	network    string
	address    string
	log        l.Logger
}

func NewClient(network, address string) *Client {
	log := l.NewLogger("debug")

	remoteConn, err := net.Dial(network, address)
	if err != nil {
		log.Error().Msgf("failed to connected to the server: %s", err.Error())
		// remoteConn.Close()
		os.Exit(-1)
	} else {
		log.Info().Msgf("connected to remote session: %s", remoteConn.RemoteAddr().String())
	}

	return &Client{
		log:        log,
		network:    network,
		address:    address,
		remoteConn: remoteConn,
	}
}

func (c *Client) Run() {
	wg := sync.WaitGroup{} // make it a part of a client so we just call go c.recv()/ go c.send()
	wg.Add(2)

	go func() {
		c.recv()
		wg.Done()
	}()

	go func() {
		c.send()
		wg.Done()
	}()

	wg.Wait()
	c.remoteConn.Close()
}

func (c *Client) recv() {
	buf := make([]byte, 4096)

	for {
		nBytes, err := c.remoteConn.Read(buf)
		if err != nil && err != io.EOF {
			c.log.Error().Msgf("failed to read from the remote connnection: %s", err.Error())
			// should we shutdown this client or handle it more gracefully?
			// Since send() function is still running, we cannot cancel recv function.
			break
		}

		c.log.Info().Msgf("%s", string(buf[:nBytes]))
	}
}

func (c *Client) send() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		c.log.Info().Msgf("sending message: %s", text)

		nBytes, err := c.remoteConn.Write([]byte(text))
		c.log.Info().Int("count", nBytes).Msg("wrote bytes")

		if err != nil {
			c.log.Error().Msgf("failed to write to remote connection: %s", err.Error())
			return
		}
	}

	if scanner.Err() != nil {
		c.log.Error().Msgf("scanner failed to read: %s", scanner.Err().Error())
	}
}
