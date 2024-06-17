package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"os"

	"github.com/isnastish/chat/pkg/client"
	"github.com/isnastish/chat/pkg/logger"
)

func main() {
	var config client.Config
	flag.StringVar(&config.Network, "network", "tcp", "Network protocol [TCP|UDP]")
	flag.StringVar(&config.Addr, "address", "localhost:8080", "Address, for example: 127.0.0.1")
	flag.IntVar(&config.RetriesCount, "retriesCount", 5, "The amount of attempts a client would make to connect to a server")
	certPemFilePath := flag.String("public-key-path", "cert.pem", "Path to the .pem file containing a public key")

	flag.Parse()

	certContents, err := os.ReadFile(*certPemFilePath)
	if err != nil {
		log.Logger.Panic("%v", err)
	}

	certPool := x509.NewCertPool()
	if parsed := certPool.AppendCertsFromPEM(certContents); !parsed {
		log.Logger.Panic("Failed to parse certificate %s", *certPemFilePath)
	}

	config.TLSConfig = &tls.Config{RootCAs: certPool}

	client := client.CreateClient(&config)
	client.Run()
}
