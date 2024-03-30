package session

import (
	_ "fmt"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestConnectionEstablished(t *testing.T) {
	s := NewSession("tcp", ":5000")

	go func() {
		conn, err := net.Dial(s.network, s.address)
		assert.Equal(t, err, nil)
		conn.Close()
	}()
	s.AcceptConnections()

	assert.Equal(t, 1, int(s.stats.ClientsJoined.Load()))
	assert.Equal(t, 1, int(s.stats.ClientsLeft.Load()))
}
