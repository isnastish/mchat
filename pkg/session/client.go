package session

import (
	"net"
	"time"
)

const (
	ClientStatus_Connected int32 = iota
	ClientStatus_Disconnected
	ClientStatus_Pending
)

type Client struct {
	conn     net.Conn
	ipAddr   string
	name     string
	password string // has to be hashed
	status   int32
	joinTime time.Time
}

func (c *Client) matchStatus(status int32) bool {
	return c.status == status
}
