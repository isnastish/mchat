package main

import (
	"flag"
	"os"

	"github.com/isnastish/chat/pkg/session"
)

func main() {
	var networkProtocol string
	var address string

	flag.StringVar(&networkProtocol, "network", "tcp", "network protocol (tcp|udp)")
	flag.StringVar(&address, "address", ":5000", "address to listen in. E.g. localhost:8080")

	flag.Parse()

	// Specify as a command line argument?
	os.Setenv("DATABASE_BACKEND", "redis")

	s := session.NewSession(networkProtocol, address)
	s.AcceptConnections()
}
