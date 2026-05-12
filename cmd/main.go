package main

import (
	"log"

	_ "time/tzdata"

	"github.com/hayakawakaki/go-racp/server"
)

func main() {
	if err := server.Start(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
