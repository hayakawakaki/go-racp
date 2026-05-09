package server

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/hayakawakaki/go-racp/internal/health"
	"github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/infra/mysql"
	"github.com/hayakawakaki/go-racp/internal/plugin"
	"github.com/hayakawakaki/go-racp/server/config"
)

// Start initializes application runtime (configuration, database connections, and structured logger),
// mounts static assets and plugins onto an HTTP mux, creates an HTTP server with sensible timeouts and header limits,
// Start initializes application resources (configuration, databases, logger, and plugin wiring)
// and runs the HTTP server on the configured port. It ensures database connections are closed
// on exit and logs any errors encountered during shutdown or from ListenAndServe.
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
		Config: cfg,
	}

	// Plugin Mounting
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	mux.HandleFunc("GET /healthz", health.New(mainDB, logsDB, logger))
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
