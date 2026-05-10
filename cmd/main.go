package main

import (
	"log"

	"github.com/hayakawakaki/go-racp/server"
)

func main() {
	if err := server.Start(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
