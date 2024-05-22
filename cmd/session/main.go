package main

import (
	"flag"
	"os"
	"strconv"

	"github.com/isnastish/chat/pkg/session"
	backend "github.com/isnastish/chat/pkg/session/backend"
)

func main() {
	config := session.SessionConfig{}

	flag.StringVar(&config.Network, "network", "tcp", "network protocol (tcp|udp)")
	flag.StringVar(&config.Addr, "address", ":5000", "address to listen in")

	flag.Parse()

	if dbBackend, exists := os.LookupEnv("DATABASE_BACKEND"); exists {
		backendType, _ := strconv.Atoi(dbBackend)
		config.BackendType = backend.BackendType(backendType)
	} else {
		config.BackendType = backend.BackendTypeMemory
	}

	s := session.NewSession(config)
	s.Run()
}
