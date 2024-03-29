package stats

import (
	"fmt"
	"sync/atomic"
)

const (
	Client int = iota
	Session
)

type Stats struct {
	MessagesReceived atomic.Int32
	MessagesSent     atomic.Int32

	// experimental, not sure how to collect those metrics
	MessagesDropped atomic.Int32

	ClientsJoined   atomic.Int32 // should only be accessible by the server
	ClientsLeft     atomic.Int32 // should only be accessible by the server
	ClientsRejoined atomic.Int32 // should only be accessible by the server
}

var names = map[int]string{
	0: "messages_received",
	1: "messages_sent",
	2: "messages_dropped",
	3: "clients_joined",
	4: "clients_left",
	5: "clients_rejoined",
}

func DisplayStats(s *Stats, who int) {
	if who == Client {
		fmt.Printf(
			"Client's stats: "+
				"\n\t|%-18s|: %d"+
				"\n\t|%-18s|: %d",

			names[0], s.MessagesReceived.Load(),
			names[1], s.MessagesSent.Load(),
		)
	} else if who == Session {
		fmt.Printf(
			"Session's stats: "+
				"\n\t|%-18s|: %d"+
				"\n\t|%-18s|: %d"+
				"\n\t|%-18s|: %d"+ // unused
				"\n\t|%-18s|: %d"+
				"\n\t|%-18s|: %d"+
				"\n\t|%-18s|: %d",

			names[0], s.MessagesReceived.Load(),
			names[1], s.MessagesSent.Load(),
			names[2], s.MessagesDropped.Load(),
			names[3], s.ClientsJoined.Load(),
			names[4], s.ClientsLeft.Load(),
			names[5], s.ClientsRejoined.Load(),
		)
	} else {
		fmt.Printf("undefined instance: %d\n", who)
	}
}
