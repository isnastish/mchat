package session

import (
	"net"
	t "time"
)

const (
	ClientStatus_Connected int32 = iota
	ClientStatus_Disconnected
	ClientStatus_Pending
)

type Client struct {
	conn      net.Conn
	name      string // rename to remoteAddr or just addr
	connected bool
	rejoined  bool
	status    int32 // atomic
	joinTime  t.Time
}
