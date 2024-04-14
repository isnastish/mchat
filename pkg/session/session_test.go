package session

import (
	bk "github.com/isnastish/chat/pkg/session/backend"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

var settings = Settings{
	NetworkProtocol: "tcp",
	Addr:            ":5000",
	BackendType:     bk.BackendType_Memory,
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

func TestClientWithNameAlreadyExists(t *testing.T) {

}
