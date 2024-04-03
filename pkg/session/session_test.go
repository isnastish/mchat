package session

import (
	"github.com/stretchr/testify/assert"
	"net"
	"testing"

	"github.com/isnastish/chat/pkg/db_backend"
)

var settings = SessionSettings{
	NetworkProtocol: "tcp",
	Addr:            ":5000",
	BackendType:     dbbackend.BackendType_Memory,
}

func TestConnectionEstablished(t *testing.T) {
	s := NewSession(&settings)

	go func() {
		conn, err := net.Dial(s.network, s.address)
		assert.Equal(t, err, nil)
		conn.Close()
	}()

	s.AcceptConnections()

	assert.Equal(t, 1, int(s.stats.ClientsJoined.Load()))
	assert.Equal(t, 1, int(s.stats.ClientsLeft.Load()))
}
