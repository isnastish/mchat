package session

import (
	"bytes"
	"net"
	"strconv"
	"strings"
	"time"
)

type test_participant struct {
	name         string
	password     string
	emailAddress string
}

var participants = []test_participant{
	{"IvanIvanov", "ThisIsI1sPassw0rd@", "example@gmail.com"},
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

var config = SessionConfig{
	Network: "tcp",
	Addr:    "127.0.0.1:5000",
	Timeout: 2 * time.Second,
}

func client(menuOption MenuOptionType,
	config *SessionConfig,
	participant *test_participant,
	onPasswordSubmittedCallback func(net.Conn) bool,
	onAcceptingMessagesCallback func(*bytes.Buffer, net.Conn) bool) {

	conn, err := net.Dial(config.Network, config.Addr)
	if err != nil {
		return
	}
	for {
		buf := bytes.NewBuffer(make([]byte, 256))
		bytesRead, err := conn.Read(buf.Bytes())
		if err != nil || bytesRead == 0 {
			return
		}

		if strings.Contains(buf.String(), string(menuMessageHeader)) {
			conn.Write([]byte(strconv.Itoa(int(menuOption))))
		} else if strings.Contains(buf.String(), string(usernameMessageContents)) {
			conn.Write([]byte(participants[0].name))
		} else if strings.Contains(buf.String(), string(emailAddressMessageContents)) {
			conn.Write([]byte(participants[0].emailAddress))
		} else if strings.Contains(buf.String(), string(passwordMessageContents)) {
			conn.Write([]byte(participants[0].password))
			if onPasswordSubmittedCallback(conn) {
				return
			}
		} else {
			if onAcceptingMessagesCallback(buf, conn) {
				return
			}
		}
	}
}
