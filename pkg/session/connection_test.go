package session

import (
	"net"
	"sync"
	"testing"
	"time"

	backend "github.com/isnastish/chat/pkg/session/backend"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

func TestAddNewConnection(t *testing.T) {
	defer goleak.VerifyNone(t)

	connMap := newConnectionMap()
	wg := sync.WaitGroup{}

	const expectedConnCount = 4
	wg.Add(expectedConnCount + 1)

	go func() {
		// wait for the server to start up
		time.Sleep(10 * time.Millisecond)
		for i := 0; i < expectedConnCount; i++ {
			go func() {
				conn, err := net.Dial("tcp", "127.0.0.1:8080")
				if err == nil {
					conn.Close()
				}
				wg.Done()
			}()
		}
		wg.Done()
	}()

	connIpAdds := make([]string, 0, expectedConnCount)

	ln, err := net.Listen("tcp", ":8080")
	if err == nil {
		connCount := 0
		for {
			conn, err := ln.Accept()
			assert.Equal(t, err, nil)
			ipAddr := conn.RemoteAddr().String()
			connMap.addConn(&Connection{
				conn:   conn,
				ipAddr: ipAddr,
				state:  Connected,
				participant: &backend.Participant{
					Name: ipAddr, // use an ip address instead of a real name
				},
			})
			connIpAdds = append(connIpAdds, ipAddr)
			connCount++
			if connCount == expectedConnCount {
				break
			}
		}
		ln.Close()
	}
	wg.Wait()

	assert.Equal(t, connMap.count(), expectedConnCount)
	for _, ipAddr := range connIpAdds {
		assert.True(t, connMap.isParticipantConnected(ipAddr))
	}

	connMap.removeConn(connIpAdds[0])
	assert.False(t, connMap.isParticipantConnected(connIpAdds[0]))
}
