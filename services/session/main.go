package main

import (
	"flag"
	"strings"

	"github.com/isnastish/chat/pkg/backend"
	"github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/session"
)

func main() {
	var config session.Config

	flag.StringVar(&config.Network, "network", "tcp", "network protocol (tcp|udp)")
	flag.StringVar(&config.Addr, "address", ":8080", "address to listen in")
	flag.DurationVar(&config.SessionTimeout, "sessionTimeout", 86400 /*24h*/, "time for the session to tear down if nobody connected")
	flag.DurationVar(&config.ParticipantTimeout, "participantTimeout", 86400, "time to be elapsed (in seconds) for the participant to be manually disconnected")
	backendType := flag.String("backend", "memory", "Backend type for persisting the data. Possible types are (redis|dynamodb|memory).")
	redisEndpoint := flag.String("redis-endpoint", "", "Redis endpoint")
	redisUsername := flag.String("redis-username", "", "Redis username")
	redisPassword := flag.String("redis-password", "", "Redis password")

	flag.Parse()

	switch strings.ToLower(*backendType) {
	case backend.BackendTypes[backend.BackendTypeRedis]:
		log.Logger.Info("Running redis backend")
		config.BackendType = backend.BackendTypeRedis
		config.RedisConfig = &backend.RedisConfig{
			Endpoint: *redisEndpoint,
			Username: *redisUsername,
			Password: *redisPassword,
		}

	case backend.BackendTypes[backend.BackendTypeDynamodb]:
		log.Logger.Info("Running dynamodb backend")
		config.BackendType = backend.BackendTypeDynamodb
		config.DynamodbConfig = &backend.DynamodbConfig{}

	case backend.BackendTypes[backend.BackendTypeMemory]:
		log.Logger.Info("Running  memory backend")
		config.BackendType = backend.BackendTypeMemory

	default:
		log.Logger.Panic("Unknown backend %s", *backendType)
	}

	s := session.CreateSession(config)
	s.Run()
}
