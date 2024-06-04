package testsetup

import (
	"bytes"
	"io"
	"net"

	"github.com/isnastish/chat/pkg/types"
)

func ClientMock(
	network string,
	addr string,
	participant *types.Participant,
	channel *types.Channel,
	onPasswordSubmittedCallback func(net.Conn) bool,
	onAcceptingMessagesCallback func(*bytes.Buffer, net.Conn) bool,
) {
	if onPasswordSubmittedCallback == nil && onAcceptingMessagesCallback == nil {
		panic("both callbacks cannot be nil, the net.Conn won't be closed")
	}

	// if onPasswordSubmittedCallback == nil {
	// 	onPasswordSubmittedCallback = func(c net.Conn) bool { return false }
	// } else if onAcceptingMessagesCallback == nil {
	// 	onAcceptingMessagesCallback = func(b *bytes.Buffer, c net.Conn) bool { return false }
	// }

	conn, err := net.Dial(network, addr)
	if err != nil {
		return
	}

	for {
		buf := bytes.NewBuffer(make([]byte, 1024))
		bytesRead, err := conn.Read(buf.Bytes())
		if err != nil {
			if err == io.EOF {
				break
			}
			return
		}

		_ = bytesRead

		// switch {
		// case strings.Contains(buf.String(), string(menuMessageHeader)):
		// 	conn.Write([]byte(strconv.Itoa(int(menuOption))))

		// case strings.Contains(buf.String(), string(usernameMessageContents)):
		// 	conn.Write([]byte(participant.username))

		// case strings.Contains(buf.String(), string(emailAddressMessageContents)):
		// 	conn.Write([]byte(participant.emailAddress))

		// case strings.Contains(buf.String(), string(passwordMessageContents)):
		// 	conn.Write([]byte(participant.password))
		// 	if onPasswordSubmittedCallback(conn) {
		// 		return
		// 	}

		// case strings.Contains(buf.String(), string(channelsNameMessageContents)):
		// 	conn.Write([]byte(channel.name))

		// case strings.Contains(buf.String(), string(channelsDescMessageContents)):
		// 	conn.Write([]byte(channel.desc))

		// default:
		// 	if onAcceptingMessagesCallback(buf, conn) {
		// 		return
		// 	}
		// }
	}
}
