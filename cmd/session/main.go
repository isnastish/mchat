package main

import (
	"flag"
	"os"
	"strconv"
	"time"

	"github.com/isnastish/chat/pkg/backend"
	"github.com/isnastish/chat/pkg/session"
)

func main() {
	var config session.Config
	config.Timeout = 15 * time.Second

	// TODO: Specify backend as a command-line argument.
	// Every config (for redis/dynamodb backends) should be passed to CreateSessio,
	// so need append the corresponding fields to SessionConfig struct
	flag.StringVar(&config.Network, "network", "tcp", "network protocol (tcp|udp)")
	flag.StringVar(&config.Addr, "address", ":5000", "address to listen in")

	flag.Parse()

	if dbBackend, exists := os.LookupEnv("DATABASE_BACKEND"); exists {
		backendType, _ := strconv.Atoi(dbBackend)
		config.BackendType = backend.BackendType(backendType)
	} else {
		config.BackendType = backend.BackendTypeRedis
	}

	s := session.CreateSession(config)
	s.Run()
}
