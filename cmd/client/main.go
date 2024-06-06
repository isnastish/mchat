// TODO: Specify port seprately from the address?
package main

import (
	"flag"

	"github.com/isnastish/chat/pkg/client"
)

func main() {
	config := client.Config{}

	flag.StringVar(&config.Network, "network", "tcp", "Network protocol [TCP|UDP]")
	flag.StringVar(&config.Addr, "address", "127.0.0.1:8080", "Address, for example: 127.0.0.1")
	flag.IntVar(&config.RetriesCount, "retriesCount", 5, "The amount of attempts a client would make to connect to a server")
	flag.Parse()

	client := client.CreateClient(&config)
	client.Run()
}
