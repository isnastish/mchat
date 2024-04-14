package session

import (
	"net"
	"time"
)

type ClientState int32

const (
	ClientState_Pending ClientState = iota + 1
	ClientState_Connected
	ClientState_Disconnected
)

type Client struct {
	Conn     net.Conn
	IpAddr   string
	Name     string
	JoinTime time.Time
	State    ClientState
}

func (c *Client) MatchState(state ClientState) bool {
	return c.State == state
}
