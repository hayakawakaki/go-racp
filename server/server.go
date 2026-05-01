package server

import (
	"fmt"
	"log"

	"github.com/hayakawakaki/go-racp/internal/infra/mysql"
	"github.com/hayakawakaki/go-racp/server/config"
)

func Start() {
	// Config Creation
	cfg := config.NewConfig()

	// MySQL Connection
	mainDB, logsDB := mysql.Connect(cfg.Env)
	defer func() {
		if err := mainDB.Close(); err != nil {
			log.Printf("close main db: %v", err)
		}
	}()
	defer func() {
		if err := logsDB.Close(); err != nil {
			log.Printf("close logs db: %v", err)
		}
	}()

	fmt.Println("Server is running...")
}
