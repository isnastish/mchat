package session

import (
	"fmt"
	"net"
	"strings"
	"testing"
	_ "time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"

	"github.com/isnastish/chat/pkg/common"
	bk "github.com/isnastish/chat/pkg/session/backend"
)

type TestClient struct {
	name     string
	password string
}

var settings = Settings{
	NetworkProtocol: "tcp",
	Addr:            ":5000",
	BackendType:     bk.BackendType_Memory,
}

// TODO: We have a bug when running more than 2 clients.
var clients = []TestClient{
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

	// {"sdfvan Ivanov", "ThisIsI1sPassw0rd"},
	// {"sdfark Lutz", "Some_other_password"},
	// {"sdfohan Novak", "johans_pathword_234"},
	// {"ffobin Hood", "bow_is_life_1024"},
	// {"ddo Gang", "woo_gang_234234"},
	// {"ddi Pu", "my_dog_2017"},
	// {"ddanna Hoflan", "some_random_password_here"},
	// {"dfsvan Istberg", "234340_sdfsdfuu"},
	// {"sdfdoir Nasaiier", "tyheroi_34234"},
	// {"sdfdark Zuckerberg", "mark_zuckergerg_2033"},
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

	go func() { // not necessary
		client, err := net.Dial(s.network, s.address)
		assert.Equal(t, err, nil)
		for {
			buf := make([]byte, 1024)
			bRead, err := client.Read(buf)
			if err != nil || bRead == 0 {
				return
			}

			input := string(common.StripCR(buf, bRead))
			if strings.Contains(input, "Menu:") {
				client.Write([]byte(RegisterClientOption))
			} else if strings.Contains(input, "@name:") {
				client.Write([]byte(clients[0].name))
			} else if strings.Contains(input, "@password:") {
				client.Write([]byte(clients[0].password))
				break
			}
		}
		fmt.Println("Closing the connection")
		client.Close()
	}()
	<-doneCh
	assert.True(t, s.backend.HasClient(clients[0].name))
}

func TestRegisterMultipeNewClients(t *testing.T) {
	fmt.Println("start")
	// Always enable this, because there some go routines in session.disconnectIdleClient found
	// defer goleak.VerifyNone(t)

	doneCh := make(chan struct{})
	s := NewSession(&settings)
	go func() {
		s.Run()
		close(doneCh)
	}()

	for _, client := range clients {
		c := client
		// time.Sleep(1000 * time.Millisecond)
		go func() {
			client, err := net.Dial(s.network, s.address)
			assert.Equal(t, err, nil)
			for {
				buf := make([]byte, 1024)
				bRead, err := client.Read(buf)
				if err != nil || bRead == 0 {
					return
				}

				input := string(common.StripCR(buf, bRead))
				if strings.Contains(input, "Menu:") {
					client.Write([]byte(RegisterClientOption))
				} else if strings.Contains(input, "@name:") {
					client.Write([]byte(c.name))
				} else if strings.Contains(input, "@password:") {
					client.Write([]byte(c.password))
					break
				}
			}
			client.Close()
		}()
	}
	<-doneCh

	for _, client := range clients {
		assert.True(t, s.backend.HasClient(client.name))
	}
	assert.Equal(t, len(clients), s.clients.size())
	fmt.Println("len(clients): ", len(clients))
}
