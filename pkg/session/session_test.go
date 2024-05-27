package session

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

type participant struct {
	name         string
	password     string
	emailAddress string
}

var network = "tcp"
var address = "127.0.0.1:5000"

var participants = []participant{
	{"Ivan Ivanov", "ThisIsI1sPassw0rd", "example@gmail.com"},
	// {"Mark Lutz", "Some_other_password"},
	// {"Johan Novak", "johans_pathword_234"},
	// {"Robin Hood", "bow_is_life_1024"},
	// {"Wo Gang", "woo_gang_234234"},
	// {"Li Pu", "my_dog_2017"},
	// {"Hanna Hoflan", "some_random_password_here"},
	// {"Ivan Istberg", "234340_sdfsdfuu"},
	// {"Noir Nasaiier", "tyheroi_34234"},
	// {"Mark Zuckerberg", "mark_zuckergerg_2033"},
}

func eatWhitespaces(buffer []byte) string {
	return strings.Trim(string(buffer), " \\r\\n\\t\\f\\v")
}

// func createClient(t *testing.T,
// 	networkProtocol, address string,
// 	testClient _Client,
// 	mainCallback func(string, net.Conn) bool,
// 	useMainCallbackInElseClause bool) {
// 	conn, err := net.Dial(networkProtocol, address)
// 	assert.Equal(t, err, nil)
// 	for {
// 		buf := make([]byte, 1024)
// 		// In order to make it more flexible, we can do multiple reads in a row,
// 		// or we can use a state machine thing.
// 		bRead, err := conn.Read(buf)
// 		if err != nil || bRead == 0 {
// 			return
// 		}

// 		input := strings.Trim(string(buf[:bRead]), " \\r\\n\\t\\f\\v")

// 		if strings.Contains(input, "Menu:") {
// 			conn.Write([]byte(strconv.Itoa(RegisterParticipant)))
// 		} else if strings.Contains(input, "@name:") {
// 			conn.Write([]byte(testClient.name))
// 		} else if strings.Contains(input, "@password:") {
// 			conn.Write([]byte(testClient.password))
// 			if !useMainCallbackInElseClause {
// 				if mainCallback(input, conn) {
// 					return
// 				}
// 			}
// 		} else {
// 			if useMainCallbackInElseClause {
// 				if mainCallback(input, conn) {
// 					return
// 				}
// 			}
// 		}
// 	}
// }

func TestShutdownSessionIfNobodyConnected(t *testing.T) {
	defer goleak.VerifyNone(t)

	config := SessionConfig{
		Network: network,
		Addr:    address,
		Timeout: 3 * time.Second,
	}

	session := NewSession(config)
	session.Run()
	assert.Equal(t, session.participantsJoined.Load(), 0)
}

func TestRegisterNewParticipant(t *testing.T) {
	defer goleak.VerifyNone(t)

	config := SessionConfig{
		Network: network,
		Addr:    address,
	}

	session := NewSession(config)
	// TODO(alx): Pull out into a function runClient()
	go func() {
		conn, err := net.Dial(config.Network, config.Addr)
		assert.Equal(t, err, nil)
		for {
			buffer := make([]byte, 256)
			bytesRead, err := conn.Read(buffer)
			if err != nil || bytesRead == 0 {
				return
			}

			input := eatWhitespaces(buffer[:bytesRead])

			if strings.Contains(input, string(menuMessageHeader)) {
				conn.Write([]byte(strconv.Itoa(RegisterParticipant)))
				fmt.Println("RegisteringNewparticipant")
			} else if strings.Contains(input, string(usernameMessageContents)) {
				conn.Write([]byte(participants[0].name))
				fmt.Println("Entering participants username")
			} else if strings.Contains(input, string(emailAddressMessageContents)) {
				conn.Write([]byte(participants[0].emailAddress))
			} else if strings.Contains(input, string(passwordMessageContents)) {
				conn.Write([]byte(participants[0].password))
				// close the connection
				conn.Close()
				fmt.Println("connection is closed")
			}
		}
	}()
	session.Run()
	assert.True(t, session.storage.HasParticipant(participants[0].name))
}

func TestAuthenticateParticipant(t *testing.T) {
	defer goleak.VerifyNone(t)
}

/*
func TestRegisterNewClient(t *testing.T) {
	defer goleak.VerifyNone(t)

	doneCh := make(chan struct{})
	s := NewSession(config)
	go func() {
		s.Run()
		close(doneCh)
	}()
	createClient(t, config.Network, config.Addr, clients[0], func(input string, conn net.Conn) bool {
		conn.Close()
		return true
	}, false)
	<-doneCh
	assert.True(t, s.backend.HasParticipant(clients[0].name))
}

func TestRegisterMultipeNewClients(t *testing.T) {
	defer goleak.VerifyNone(t)

	doneCh := make(chan struct{})
	s := NewSession(config)
	go func() {
		s.Run()
		close(doneCh)
	}()

	for _, client := range clients {
		c := client
		go createClient(t, config.Network, config.Addr, c, func(input string, conn net.Conn) bool {
			conn.Close()
			return true
		}, false)
	}
	<-doneCh

	for _, client := range clients {
		assert.True(t, s.backend.HasParticipant(client.name))
	}
}

func TestSecondClientReceivedMessages(t *testing.T) {
	defer goleak.VerifyNone(t)
	message := "hello!"

	doneCh := make(chan struct{})
	s := NewSession(config)
	go func() {
		s.Run()
		close(doneCh)
	}()
	go createClient(t, config.Network, config.Addr, clients[0], func(input string, conn net.Conn) bool {
		time.Sleep(100 * time.Millisecond) // Do we need to sleep?
		conn.Write([]byte(message))
		conn.Close()
		return true
	}, false)

	go createClient(t, config.Network, config.Addr, clients[1], func(input string, conn net.Conn) bool {
		assert.True(t, strings.Contains(input, message))
		conn.Close()
		return true
	}, true)

	<-doneCh
}
*/
