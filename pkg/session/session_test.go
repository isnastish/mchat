package session

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"

	"github.com/isnastish/chat/pkg/common"
	bk "github.com/isnastish/chat/pkg/session/backend"
)

type _Client struct {
	name     string
	password string
}

var settings = Settings{
	NetworkProtocol: "tcp",
	Addr:            ":5000",
	BackendType:     bk.BackendType_Memory,
}

var clients = []_Client{
	{"Ivan Ivanov", "ThisIsI1sPassw0rd"},
	{"Mark Lutz", "Some_other_password"},
	{"Johan Novak", "johans_pathword_234"},
	{"Robin Hood", "bow_is_life_1024"},
	{"Wo Gang", "woo_gang_234234"},
	{"Li Pu", "my_dog_2017"},
	{"Hanna Hoflan", "some_random_password_here"},
	{"Ivan Istberg", "234340_sdfsdfuu"},
	{"Noir Nasaiier", "tyheroi_34234"},
	{"Mark Zuckerberg", "mark_zuckergerg_2033"},
}

func createClient(t *testing.T,
	networkProtocol, address string,
	testClient _Client,
	mainCallback func(string, net.Conn) bool,
	useMainCallbackInElseClause bool) {
	conn, err := net.Dial(networkProtocol, address)
	assert.Equal(t, err, nil)
	for {
		buf := make([]byte, 1024)
		// In order to make it more flexible, we can do multiple reads in a row,
		// or we can
		bRead, err := conn.Read(buf)
		if err != nil || bRead == 0 {
			return
		}

		input := string(common.StripCR(buf, bRead))

		if strings.Contains(input, "Menu:") {
			conn.Write([]byte(RegisterClientOption))
		} else if strings.Contains(input, "@name:") {
			conn.Write([]byte(testClient.name))
		} else if strings.Contains(input, "@password:") {
			conn.Write([]byte(testClient.password))
			if !useMainCallbackInElseClause {
				if mainCallback(input, conn) {
					return
				}
			}
		} else {
			if useMainCallbackInElseClause {
				if mainCallback(input, conn) {
					return
				}
			}
		}
	}
}

func TestConnectionEstablished(t *testing.T) {
	defer goleak.VerifyNone(t)

	s := NewSession(&settings)
	go func() {
		conn, err := net.Dial(s.network, s.address)
		assert.Equal(t, err, nil)
		conn.Close()
	}()
	s.Run()
}

func TestRegisterNewClient(t *testing.T) {
	defer goleak.VerifyNone(t)

	doneCh := make(chan struct{})
	s := NewSession(&settings)
	go func() {
		s.Run()
		close(doneCh)
	}()
	createClient(t, settings.NetworkProtocol, settings.Addr, clients[0], func(input string, conn net.Conn) bool {
		conn.Close()
		return true
	}, false)
	<-doneCh
	assert.True(t, s.backend.HasClient(clients[0].name))
}

func TestRegisterMultipeNewClients(t *testing.T) {
	defer goleak.VerifyNone(t)

	doneCh := make(chan struct{})
	s := NewSession(&settings)
	go func() {
		s.Run()
		close(doneCh)
	}()

	for _, client := range clients {
		c := client
		go createClient(t, settings.NetworkProtocol, settings.Addr, c, func(input string, conn net.Conn) bool {
			conn.Close()
			return true
		}, false)
	}
	<-doneCh

	for _, client := range clients {
		assert.True(t, s.backend.HasClient(client.name))
	}
	assert.Equal(t, len(clients), s.clients.size())
}

func TestSecondClientReceivedMessages(t *testing.T) {
	defer goleak.VerifyNone(t)
	message := "hello!"

	doneCh := make(chan struct{})
	s := NewSession(&settings)
	go func() {
		s.Run()
		close(doneCh)
	}()
	go createClient(t, settings.NetworkProtocol, settings.Addr, clients[0], func(input string, conn net.Conn) bool {
		time.Sleep(100 * time.Millisecond) // Do we need to sleep?
		conn.Write([]byte(message))
		conn.Close()
		return true
	}, false)

	go createClient(t, settings.NetworkProtocol, settings.Addr, clients[1], func(input string, conn net.Conn) bool {
		assert.True(t, strings.Contains(input, message))
		conn.Close()
		return true
	}, true)

	<-doneCh
}
