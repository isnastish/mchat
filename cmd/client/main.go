package main

import (
	"flag"
	"fmt"
	"github.com/isnastish/chat/pkg/client"
)

func main() {
	var network string
	var address string

	flag.StringVar(&network, "network", "tcp", "Network protocol [TCP|UDP]")
	flag.StringVar(&address, "address", "127.0.0.1:5000", "Address, for example: localhost:8080")

	flag.Parse()

	c, err := client.NewClient(network, address)
	if err != nil {
		fmt.Printf("failed to create a client: %s\n", err.Error())
		return
	}

	c.Run()
}
