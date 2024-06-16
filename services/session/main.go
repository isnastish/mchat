package main

import (
	"crypto/tls"
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
	certFile := flag.String("cert", "cert.pem", "Path to the cert.pem file")
	keyFile := flag.String("key", "key.pem", "Path to the key.pem file")
	backendType := flag.String("backend", "memory", "Backend type for persisting the data. Possible types are (redis|dynamodb|memory).")
	redisEndpoint := flag.String("redis-endpoint", "", "Redis endpoint")
	redisUsername := flag.String("redis-username", "", "Redis username")
	redisPassword := flag.String("redis-password", "", "Redis password")

	flag.Parse()

	*backendType = strings.ToLower(*backendType)
	switch *backendType {
	case backend.BackendTypes[backend.BackendTypeRedis]:
		config.BackendType = backend.BackendTypeRedis
		config.RedisConfig = &backend.RedisConfig{
			Endpoint: *redisEndpoint,
			Username: *redisUsername,
			Password: *redisPassword,
		}
		log.Logger.Info("Running redis backend")

	case backend.BackendTypes[backend.BackendTypeDynamodb]:
		config.BackendType = backend.BackendTypeDynamodb
		config.DynamodbConfig = &backend.DynamodbConfig{}
		log.Logger.Info("Running dynamodb backend")

	case backend.BackendTypes[backend.BackendTypeMemory]:
		log.Logger.Info("Running  memory backend")
		config.BackendType = backend.BackendTypeMemory

	default:
		log.Logger.Panic("Unknown backend %s", *backendType)
	}

	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Logger.Panic("Failed to parse certificates %v", err)
	}

	config.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}

	s := session.CreateSession(&config)
	s.Run()
}
