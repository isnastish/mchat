package main

import (
	"flag"
	"os"
	"strconv"

	"github.com/isnastish/chat/pkg/db_backend"
	"github.com/isnastish/chat/pkg/session"
)

func main() {
	settings := session.SessionSettings{}

	flag.StringVar(&settings.NetworkProtocol, "network", "tcp", "network protocol (tcp|udp)")
	flag.StringVar(&settings.Addr, "address", ":5000", "address to listen in")

	flag.Parse()

	if dbBackend, exists := os.LookupEnv("DATABASE_BACKEND"); exists {
		settings.BackendType, _ = strconv.Atoi(dbBackend)
	} else {
		settings.BackendType = dbbackend.BackendType_Memory
	}

	s := session.NewSession(&settings)
	s.AcceptConnections()
}
