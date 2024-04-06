package main

import (
	"flag"
	"fmt"
	"github.com/isnastish/chat/pkg/client"
)

func main() {
	var network string
	var address string

	// 127.0.0.1:8200

	flag.StringVar(&network, "network", "tcp", "Network protocol [TCP|UDP]")
	flag.StringVar(&address, "address", "127.0.0.1:8200", "Address, for example: localhost:8080")

	flag.Parse()

	c, err := client.NewClient(network, address)
	if err != nil {
		fmt.Printf("failed to create a client: %s\n", err.Error())
		return
	}

	c.Run()
}
