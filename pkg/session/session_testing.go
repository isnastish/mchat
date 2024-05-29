package session

import (
	"bytes"
	"net"
	"strconv"
	"strings"
	"time"
)

type test_participant struct {
	username     string
	password     string
	emailAddress string
}

var participants = []test_participant{
	{"IvanIvanov", "ThisIsI1sPassw0rd@", "example@gmail.com"},
	{"MarkLutz", "Some_other_password@234", "mark@mail.ru"},
	// {"JohanNovak", "johans_pathword_234"},
	// {"RobinHood", "bow_is_life_1024"},
	// {"WoGang", "woo_gang_234234"},
	// {"LiPu", "my_dog_2017"},
	// {"HannaHoflan", "some_random_password_here"},
	// {"IvanIstberg", "234340_sdfsdfuu"},
	// {"NoirNasaiier", "tyheroi_34234"},
	// {"MarkZuckerberg", "mark_zuckergerg_2033"},
}

var config = SessionConfig{
	Network: "tcp",
	Addr:    "127.0.0.1:5000",
	Timeout: 2 * time.Second,
}

func client(menuOption MenuOptionType,
	config SessionConfig,
	participant test_participant,
	onPasswordSubmittedCallback func(net.Conn) bool,
	onAcceptingMessagesCallback func(*bytes.Buffer, net.Conn) bool) {

	if onPasswordSubmittedCallback == nil && onAcceptingMessagesCallback == nil {
		panic("both callbacks cannot be nil, the net.Conn won't be closed")
	}

	if onPasswordSubmittedCallback == nil {
		onPasswordSubmittedCallback = func(c net.Conn) bool { return false }
	} else if onAcceptingMessagesCallback == nil {
		onAcceptingMessagesCallback = func(b *bytes.Buffer, c net.Conn) bool { return false }
	}

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
			conn.Write([]byte(participant.username))
		} else if strings.Contains(buf.String(), string(emailAddressMessageContents)) {
			conn.Write([]byte(participant.emailAddress))
		} else if strings.Contains(buf.String(), string(passwordMessageContents)) {
			conn.Write([]byte(participant.password))
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
