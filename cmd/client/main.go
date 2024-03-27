package main

import (
	"flag"
	"github.com/isnastish/chat/pkg/client"
)

func main() {
	// Each client should have an incoming and outgonig channels where he sends/receives messages
	// incomingMessagesCh := make(chan Message)
	// outgoinMessagesCh := make(chan Message)

	// _ = incomingMessagesCh
	// _ = outgoinMessagesCh

	var network string
	var address string

	flag.StringVar(&network, "network", "tcp", "Network protocol [TCP|UDP]")
	flag.StringVar(&address, "address", "127.0.0.1:5000", "Address, for example: localhost:8080")

	flag.Parse()

	c := client.NewClient(network, address)
	c.Run()
}
