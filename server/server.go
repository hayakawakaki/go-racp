package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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

	// Signal context - cancelled on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

	// Run the server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Block until either a shutdown signal arrives or the server fails to start.
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErr:
		logger.Error("server failed", "error", err)
	}

	// Stop accepting new requests and let in-flight ones drain (bounded).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
	logger.Info("server stopped")
}
