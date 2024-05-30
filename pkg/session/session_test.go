package session

import (
	"bytes"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

func TestRegisterNewParticipant(t *testing.T) {
	defer goleak.VerifyNone(t)

	session := NewSession(config)
	go client(
		RegisterParticipant,
		config,
		&participants[0],
		nil,
		func(c net.Conn) bool { c.Close(); return true },
		nil,
	)
	session.Run()
	assert.True(t, session.storage.HasParticipant(participants[0].username))
	assert.Equal(t, session.connections.count(), 0)
}

func TestFailedToValidateCredentials(t *testing.T) {
	defer goleak.VerifyNone(t)

	session := NewSession(config)
	for i := 0; i < 3; i++ {
		invalid_participant := participants[0]
		if i == 0 {
			invalid_participant.username = "#this_is_an_invalid_username#"
		} else if i == 1 {
			invalid_participant.emailAddress = "#this_email_address@_@_is_invalid"
		} else {
			invalid_participant.password = "this_password_is_invalid"
		}

		index := i

		go client(
			RegisterParticipant,
			config,
			&invalid_participant,
			nil,
			nil,
			func(buf *bytes.Buffer, c net.Conn) bool {
				if index == 0 {
					assert.True(t, strings.Contains(buf.String(), string(usernameValidationFailedMessageContents)))
				} else if index == 1 {
					assert.True(t, strings.Contains(buf.String(), string(emailAddressValidationFailedMessageContents)))
				} else {
					assert.True(t, strings.Contains(buf.String(), string(passwordValidationFailedMessageContents)))
				}
				c.Close()
				return true
			},
		)
	}
	session.Run()
	assert.Equal(t, len(session.storage.GetParticipantList()), 0)
	assert.Equal(t, session.connections.count(), 0)
}

func TestChannelCreation(t *testing.T) {
	// NOTE(alx): Creating channels is not supported yet, since the client has to be registered,
	// or authenticated in order to have the rights to create channels.
	// And we haven't agreed on the command to invoke the menu either (probably :menu)
	defer goleak.VerifyNone(t)
}

func TestAuthenticateParticipant(t *testing.T) {
	defer goleak.VerifyNone(t)

	wg := sync.WaitGroup{}
	message := []byte("Authentication succeeded!")
	session := NewSession(config)

	wg.Add(2)
	go func() {
		session.Run()
		wg.Done()
	}()

	client(
		RegisterParticipant,
		config,
		&participants[0],
		nil,
		func(c net.Conn) bool { c.Close(); return true },
		nil,
	)

	go func() {
		client(
			AuthenticateParticipant,
			config,
			&participants[0],
			nil,
			nil,
			func(buf *bytes.Buffer, c net.Conn) bool {
				c.Write([]byte(message))
				c.Close()
				return true
			},
		)
		wg.Done()
	}()
	wg.Wait()
	assert.True(t, session.storage.HasParticipant(participants[0].username))

	chatHistory := session.storage.GetChatHistory()

	assert.Equal(t, len(chatHistory), 1)
	assert.Equal(t, chatHistory[0].Contents, message)
}

/*func TestRegisterNewClient(t *testing.T) {
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
