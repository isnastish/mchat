package main

import (
	"flag"
	"os"
	"strconv"

	bk "github.com/isnastish/chat/pkg/backend"
	"github.com/isnastish/chat/pkg/session"
)

func main() {
	settings := session.Settings{}

	flag.StringVar(&settings.NetworkProtocol, "network", "tcp", "network protocol (tcp|udp)")
	flag.StringVar(&settings.Addr, "address", ":5000", "address to listen in")

	flag.Parse()

	if dbBackend, exists := os.LookupEnv("DATABASE_BACKEND"); exists {
		settings.BackendType, _ = strconv.Atoi(dbBackend)
	} else {
		settings.BackendType = bk.BackendType_Memory
	}

	s := session.NewSession(&settings)
	s.AcceptConnections()
}
