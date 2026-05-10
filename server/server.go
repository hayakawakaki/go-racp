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
	"github.com/hayakawakaki/go-racp/internal/infra/mailer"
	"github.com/hayakawakaki/go-racp/internal/infra/mysql"
	"github.com/hayakawakaki/go-racp/internal/plugin"
	"github.com/hayakawakaki/go-racp/server/config"
)

// Start initializes application runtime (configuration, database connections, and structured logger)
func Start() {
	// Config Creation
	cfg := config.NewConfig()

	// Logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Mailer (constructed before any defer-cleanup steps so a failure here can log.Fatal cleanly)
	mailClient, err := mailer.NewClient(cfg.Env.SMTPHost, cfg.Env.SMTPPort)
	if err != nil {
		log.Fatalf("init mailer: %v", err)
	}

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
	defer func() {
		if err := mailClient.Close(); err != nil {
			log.Printf("close mailer: %v", err)
		}
	}()

	in := &infra.Infra{
		MainDB: mainDB,
		LogDB:  logsDB,
		Logger: logger,
		Mailer: mailClient,
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
