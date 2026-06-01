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

type gateError struct {
	Error string `json:"error"`
}

func APIKeyGate(validator Validator, limiters map[string]*security.RateLimiter, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawKey, ok := bearerToken(r)
			if !ok {
				writeGateError(w, http.StatusUnauthorized, "missing or malformed api key")
				return
			}

			key, err := validator.Validate(r.Context(), rawKey)
			if err != nil {
				writeGateError(w, http.StatusUnauthorized, "invalid api key")
				return
			}

			limiter, found := limiters[key.RateTier]
			if !found {
				logger.Warn("apikey gate: tier has no limiter", "tier", key.RateTier)
				next.ServeHTTP(w, r)
				return
			}

			delay, allowed := limiter.Allow(strconv.FormatInt(key.ID, 10))
			if !allowed {
				seconds := max(int(delay.Round(time.Second).Seconds()), 1)
				w.Header().Set("Retry-After", strconv.Itoa(seconds))
				writeGateError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
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

func writeGateError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Cache-Control", "no-store")
	_ = httpx.WriteJSON(w, status, gateError{Error: message})
}
