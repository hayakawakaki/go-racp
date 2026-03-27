package main

import (
	"log"

	"github.com/hayakawakaki/go-racp/server"
)

func main() {
	srv, err := server.NewServer()
	if err != nil {
		log.Fatal(err)
	}
	srv.Run()
}
