package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: netspam <NODENAME> <PORT>\n")
		os.Exit(1)
	}

	id := os.Args[1]
	port, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid port (must be numeric): %s", err.Error())
		os.Exit(1)
	}

	pd := NewPeerDiscovery(id, "netspam", nil)
	pd.StartAdvertising(port)
	defer pd.StopAdvertising()

	mb := NewMessageBus()
	pd.OnPeerFound = mb.AddPeer

	log.SetPrefix(fmt.Sprintf("%s: ", id))
	con := Console{}

	go func() {
		mb.Start(context.Background(), port, con, nil)
	}()

	go func() {
		pd.Watch(context.Background())
	}()

	con.Run(mb.Send)
}

type Console struct{}

func (c Console) HandleMessage(fromID string, msg []byte) {
	if fromID == "" {
		return
	}

	log.Printf("RECEIVED FROM %s: %s\n", fromID, string(msg))
}

func (c Console) Run(sender func([]byte) error) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		s := scanner.Text()
		log.Printf("SENDING '%s'\n", s)

		err := sender([]byte(s))
		if err != nil {
			log.Printf("ERROR SENDING '%s': %s\n", s, err)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Println("ERROR reading standard input:", err)
	}
}
