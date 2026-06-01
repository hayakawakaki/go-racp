package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/apikey/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/security"
)

type Validator interface {
	Validate(ctx context.Context, rawKey string) (*domain.APIKey, error)
}

type GateConfig struct {
	Validator Validator
	Limiters  map[string]*security.RateLimiter
	Fallback  *security.RateLimiter
	Invalid   *security.RateLimiter
	Logger    *slog.Logger
}

func APIKeyGate(cfg GateConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawKey, ok := bearerToken(r)
			if !ok {
				rejectInvalid(w, r, cfg.Invalid)
				return
			}

			key, err := cfg.Validator.Validate(r.Context(), rawKey)
			if err != nil {
				rejectInvalid(w, r, cfg.Invalid)
				return
			}

			limiter := cfg.Limiters[key.RateTier]
			if limiter == nil {
				cfg.Logger.Warn("apikey gate: tier has no limiter, applying fallback", "tier", key.RateTier)
				limiter = cfg.Fallback
			}

			if delay, allowed := limiter.Allow(strconv.FormatInt(key.ID, 10)); !allowed {
				w.Header().Set("Retry-After", retryAfterSeconds(delay))
				writeGateError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func rejectInvalid(w http.ResponseWriter, r *http.Request, invalid *security.RateLimiter) {
	if delay, ok := invalid.AllowRequest(r); !ok {
		w.Header().Set("Retry-After", retryAfterSeconds(delay))
		writeGateError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}

	writeGateError(w, http.StatusUnauthorized, "invalid api key")
}

func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if len(header) < len("Bearer ") || !strings.EqualFold(header[:len("Bearer ")], "Bearer ") {
		return "", false
	}

	token := strings.TrimSpace(header[len("Bearer "):])
	if token == "" {
		return "", false
	}

	return token, true
}

func retryAfterSeconds(delay time.Duration) string {
	return strconv.Itoa(max(int(delay.Round(time.Second).Seconds()), 1))
}

func writeGateError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Cache-Control", "no-store")
	_ = httpx.WriteJSONError(w, status, message)
}
