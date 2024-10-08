package main

import (
	"context"
	"fmt"
	"os"

	"github.com/coder/websocket"
)

func main() {
	conn, _, err := websocket.Dial(context.Background(), "ws://localhost:8000/ws", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: dial: %s\n", err)
		os.Exit(1)
	}
	for {
		typ, msg, err := conn.Read(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: read: %s\n", err)
			os.Exit(1)
		}
		if typ != websocket.MessageText {
			fmt.Fprintf(os.Stderr, "ERROR: wrong message type\n")
			continue
		}
		fmt.Println(string(msg))
	}
}
