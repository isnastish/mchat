// TODO: Specify backend as a command-line argument.
// Every config (for redis/dynamodb backends) should be passed to CreateSessio,
// so need append the corresponding fields to SessionConfig struct
// TODO: Rethink whether we need a session timeout at all.
package main

import (
	"flag"
	"os"
	"strconv"

	"github.com/isnastish/chat/pkg/backend"
	"github.com/isnastish/chat/pkg/session"
)

func main() {
	var config session.Config

	flag.StringVar(&config.Network, "network", "tcp", "network protocol (tcp|udp)")
	flag.StringVar(&config.Addr, "address", ":5000", "address to listen in")
	flag.Int64Var(&config.SessionTimeout, "sessionTimeout", 86400 /*24h*/, "time for the session to tear down if nobody connected")
	flag.Int64Var(&config.ParticipantTieout, "participantTimeout", 86400, "time to be elapsed (in seconds) for the participant to be manually disconnected")

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
