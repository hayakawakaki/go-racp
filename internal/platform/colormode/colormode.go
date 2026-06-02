package colormode

import (
	"context"
	"net/http"
)

const cookieName = "theme"

const (
	dark  = "dark"
	light = "light"
)

type ctxKey int

const modeKey ctxKey = 0

func IsDark(ctx context.Context) bool {
	mode, _ := ctx.Value(modeKey).(string)
	return mode != light
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mode := dark
		if c, err := r.Cookie(cookieName); err == nil && c.Value == light {
			mode = light
		}
		ctx := context.WithValue(r.Context(), modeKey, mode)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
