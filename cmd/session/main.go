package main

import (
	"flag"
	"github.com/isnastish/chat/pkg/session"
)

func main() {
	var networkProtocol string
	var address string

	flag.StringVar(&networkProtocol, "network", "tcp", "network protocol (tcp|udp)")
	flag.StringVar(&address, "address", ":5000", "address to listen in. E.g. localhost:8080")

	flag.Parse()

	s := session.NewSession(networkProtocol, address)
	s.AcceptConnections()
}
