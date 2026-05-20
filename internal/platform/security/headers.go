package security

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/hayakawakaki/go-racp/server/config"
)

type HeadersOptions struct {
	Cfg    config.SecurityConfig
	Secure bool
}

func Headers(opts HeadersOptions) func(http.Handler) http.Handler {
	csp := buildCSP(opts.Cfg)
	hsts := buildHSTS(opts.Cfg)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Security-Policy", csp)
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), interest-cohort=()")
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Cross-Origin-Resource-Policy", "same-origin")
			if opts.Secure && hsts != "" {
				h.Set("Strict-Transport-Security", hsts)
			}
			next.ServeHTTP(w, r)
		})
	}
}

const cspSelf = "'self'"

func buildCSP(cfg config.SecurityConfig) string {
	script := slices.Concat([]string{cspSelf}, cfg.CSPExtraScriptSrc)
	style := slices.Concat([]string{cspSelf, "'unsafe-inline'"}, cfg.CSPExtraStyleSrc)
	img := slices.Concat([]string{cspSelf, "data:"}, cfg.CSPExtraImgSrc)
	parts := []string{
		"default-src " + cspSelf,
		"script-src " + strings.Join(script, " "),
		"style-src " + strings.Join(style, " "),
		"img-src " + strings.Join(img, " "),
		"font-src " + cspSelf + " data:",
		"connect-src " + cspSelf,
		"frame-ancestors 'none'",
		"form-action " + cspSelf,
		"base-uri " + cspSelf,
		"object-src 'none'",
		"upgrade-insecure-requests",
	}

	return strings.Join(parts, "; ")
}

func buildHSTS(cfg config.SecurityConfig) string {
	if cfg.HSTSMaxAge <= 0 {
		return ""
	}
	out := "max-age=" + strconv.Itoa(cfg.HSTSMaxAge)
	if cfg.HSTSIncludeSubdomains {
		out += "; includeSubDomains"
	}
	if cfg.HSTSPreload {
		out += "; preload"
	}

	return out
}
