package server

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"syscall"
	"time"

	app "github.com/hayakawakaki/go-racp/internal/features/account/app/self"
	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	accinfra "github.com/hayakawakaki/go-racp/internal/features/account/infra"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/infra/mailer"
	"github.com/hayakawakaki/go-racp/internal/infra/mysql"
	"github.com/hayakawakaki/go-racp/internal/infra/postgres"
	actiontokenapp "github.com/hayakawakaki/go-racp/internal/platform/actiontoken/app"
	actiontokeninfra "github.com/hayakawakaki/go-racp/internal/platform/actiontoken/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/health"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/metric"
	"github.com/hayakawakaki/go-racp/internal/platform/plugin"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/internal/platform/security"
	"github.com/hayakawakaki/go-racp/internal/platform/theme"
	"github.com/hayakawakaki/go-racp/internal/platform/worker"
	"github.com/hayakawakaki/go-racp/server/config"
)

// Start initializes application runtime (configuration, database connections, and structured logger)
func Start() error {
	// Config Creation
	cfg := config.NewConfig()

	csrfSecret := decodeCSRFSecret(cfg.Env.CSRFSecret)

	// Logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	logger.Info("theme: active", "name", cfg.App.General.Theme)

	// Mailer (constructed before any defer-cleanup steps so a failure here can log.Fatal cleanly)
	mailClient, err := mailer.NewClient(cfg.Env.SMTPHost, cfg.Env.SMTPPort, cfg.Env.Mode != "development")
	if err != nil {
		log.Fatalf("init mailer: %v", err)
	}
	smtpMailer := mailer.NewSMTPMailer(mailClient, cfg.App.Mailer.FromAddress, logger)

	// Signal context - cancelled on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// MySQL Connection
	mainDB, logsDB := mysql.Connect(cfg.Env)
	defer func() {
		if closeErr := mainDB.Close(); closeErr != nil {
			log.Printf("close main db: %v", closeErr)
		}
	}()
	defer func() {
		if closeErr := logsDB.Close(); closeErr != nil {
			log.Printf("close logs db: %v", closeErr)
		}
	}()

	cpPool := postgres.Connect(cfg.Env)
	defer cpPool.Close()

	defer func() {
		if closeErr := smtpMailer.Close(); closeErr != nil {
			log.Printf("close mailer: %v", closeErr)
		}
	}()

	actionTokenRepo := actiontokeninfra.NewPostgresRepository(cpPool)
	tokenMgr := actiontokenapp.NewManager(actionTokenRepo)

	in := &infra.Infra{
		MainDB:       mainDB,
		LogDB:        logsDB,
		DB:           cpPool,
		Logger:       logger,
		Mailer:       smtpMailer,
		TokenManager: tokenMgr,
		Config:       cfg,
		Roles:        domain.NewRoleResolver(cfg.App.UserRoles),
		ShutdownCtx:  ctx,
	}

	in.Metric = metric.Start(ctx, metric.Deps{
		MainDB: mainDB,
		Pool:   cpPool,
		Logger: logger,
		Config: cfg.App,
	})

	accessCfg := config.ProcessAccessConfig(theme.ActiveAccessYAML)
	layout := httpx.Layout{GeneralConfig: cfg.App.General}

	limiters, err := buildRateLimiters(in.ShutdownCtx, accessCfg, cfg.App.Security.TrustedProxyCIDRs, logger, layout)
	if err != nil {
		return fmt.Errorf("server.Start: %w", err)
	}

	sessionRepo := accinfra.NewSessionRepository(cpPool)
	loginAttemptsRepo := accinfra.NewLoginAttemptsRepository(cpPool)
	go worker.Run(ctx, logger, buildWorkerJobs(cfg.App.Retention, sessionRepo, loginAttemptsRepo)...)

	sessSvc := app.NewSessionService(sessionRepo, cfg.App.TTL.Session)
	secure := cfg.Env.Mode != "development"
	withSession := middleware.WithSession(sessSvc, logger, secure)

	userRepo := accinfra.NewRepository(mainDB)
	reg := routes.NewRegistry(
		accessCfg,
		limiters,
		in.Roles,
		sessSvc,
		userRepo,
		logger,
		secure,
		cfg.App.Auth.AllowTempBannedLogin,
		layout,
	)

	// Plugin Mounting
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	mux.HandleFunc("GET /healthz", health.New(mainDB, logsDB, postgres.NewHealthPinger(cpPool), logger))
	plugin.MountAll(reg, mux, in)
	reg.Finalize()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpx.Render404(w, r, logger, layout)
	})

	sessionFingerprint := func(ctx context.Context) ([]byte, bool) {
		session, ok := middleware.SessionFromContext(ctx)
		if !ok {
			return nil, false
		}

		return session.TokenHash[:], true
	}

	var handler http.Handler = mux
	for _, p := range plugin.Middlewares() {
		handler = p.Middleware(in, handler)
	}

	handler = security.CSRF(security.CSRFOptions{
		Secret:           csrfSecret,
		GetSessionFinger: sessionFingerprint,
		Secure:           secure,
	})(handler)
	handler = http.HandlerFunc(withSession(handler.ServeHTTP))

	// Security origin/referer check
	handler = security.Origin(security.OriginOptions{
		TrustedOrigins: cfg.App.Security.TrustedOrigins,
	})(handler)

	// Security headers check
	handler = security.Headers(security.HeadersOptions{
		Cfg:    cfg.App.Security,
		Secure: secure,
	})(handler)

	addr := fmt.Sprintf(":%d", cfg.Env.AppPort)
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}

	if err := runHTTP(ctx, srv, stop, logger); err != nil {
		return err
	}
	logger.Info("server stopped")
	return nil
}

func decodeCSRFSecret(raw string) []byte {
	secret, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		log.Fatalf("CSRF_SECRET must be base64-encoded (generate via 'openssl rand -base64 32'): %v", err)
	}
	if len(secret) < 32 {
		log.Fatalf("CSRF_SECRET must decode to >= 32 bytes (got %d)", len(secret))
	}

	return secret
}

func buildWorkerJobs(cfg config.RetentionConfig, sessions *accinfra.SessionRepository, loginAttempts *accinfra.LoginAttemptsRepository) []worker.Job {
	return []worker.Job{
		{
			Name:     "cp_sessions",
			Interval: cfg.SweepInterval,
			Fn:       sessions.DeleteExpired,
		},
		{
			Name:     "cp_login_attempts",
			Interval: cfg.SweepInterval,
			Fn: func(ctx context.Context) (int64, error) {
				return loginAttempts.DeleteOlderThan(ctx, time.Now().Add(-cfg.LoginAttempts))
			},
		},
	}
}

func buildRateLimiters(ctx context.Context, accessCfg config.AccessConfig, trustedProxies []string, logger *slog.Logger, layout httpx.Layout) (map[string]*security.RateLimiter, error) {
	limiters := map[string]*security.RateLimiter{}
	reject := rateLimitRejectFunc(layout, logger)

	for groupName, actions := range accessCfg {
		for actionName, entry := range actions {
			if entry.RateLimit == nil {
				continue
			}

			tag := groupName + "." + actionName

			opts := security.RateLimiterOptions{
				Name:           tag,
				Rule:           *entry.RateLimit,
				TrustedProxies: trustedProxies,
				Logger:         logger,
				Reject:         reject,
			}
			if !isPublicEntry(entry) {
				opts.KeyFunc = sessionUserKey
			}

			limiter, err := security.NewRateLimiter(opts)
			if err != nil {
				return nil, fmt.Errorf("server.buildRateLimiters: %w", err)
			}

			go limiter.Run(ctx)
			limiters[tag] = limiter
		}
	}

	return limiters, nil
}

func rateLimitRejectFunc(layout httpx.Layout, logger *slog.Logger) security.RejectFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			httpx.Render429(w, r, logger, layout)
			return
		}

		http.Error(w, "you are being rate limited", http.StatusTooManyRequests)
	}
}

func isPublicEntry(entry config.Entry) bool {
	return slices.Contains(entry.Roles, domain.RolePublic.Name)
}

func sessionUserKey(r *http.Request) string {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok || session == nil {
		return ""
	}

	return strconv.Itoa(session.UserID)
}

func runHTTP(ctx context.Context, srv *http.Server, stop func(), logger *slog.Logger) error {
	// Run the server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Block until either a shutdown signal arrives or the server fails to start.
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
		stop()
	case err := <-serverErr:
		logger.Error("server failed", "error", err)
		return fmt.Errorf("server failed: %w", err)
	}

	// Stop accepting new requests and let in-flight ones drain (bounded).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		return fmt.Errorf("shutdown failed: %w", err)
	}
	return nil
}
