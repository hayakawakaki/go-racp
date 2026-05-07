// Package server wires together the application's infrastructure, plugins, and
// HTTP server lifecycle.
package server

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/infra/mysql"
	"github.com/hayakawakaki/go-racp/internal/plugin"
	"github.com/hayakawakaki/go-racp/server/config"
)

// Start boots the application. It:
//  1. Loads configuration via config.NewConfig.
//  2. Opens the main and logging MySQL connections.
//  3. Builds a structured slog logger writing to stderr.
//  4. Calls plugin.MountAll to register all plugin routes onto a new ServeMux.
//  5. Starts an http.Server with conservative read/write timeouts and blocks
//     until the server exits, logging any error.
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

	// Logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	in := &infra.Infra{
		MainDB: mainDB,
		LogDB:  logsDB,
		Logger: logger,
	}

	// Plugin Mounting
	mux := http.NewServeMux()
	plugin.MountAll(mux, in)

	addr := fmt.Sprintf(":%d", cfg.Env.AppPort)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}

	logger.Info("server starting", "addr", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Printf("http: %v", err)
	}
}
