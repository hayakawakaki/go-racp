package security

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
)

type OriginOptions struct {
	OpenRoutes     *RouteMatcher
	TrustedOrigins []string
}

func Origin(opts OriginOptions) func(http.Handler) http.Handler {
	trusted := make(map[string]struct{}, len(opts.TrustedOrigins))
	for _, o := range opts.TrustedOrigins {
		trusted[strings.ToLower(strings.TrimSuffix(o, "/"))] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if opts.OpenRoutes.Allows(r) {
				next.ServeHTTP(w, r)
				return
			}
			if isSafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			if !originMatches(r, trusted) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isSafeMethod(m string) bool {
	return slices.Contains([]string{http.MethodGet, http.MethodHead, http.MethodOptions}, m)
}

func originMatches(r *http.Request, trusted map[string]struct{}) bool {
	if r.Header.Get("Sec-Fetch-Site") == "same-origin" {
		return true
	}
	expectedHost := strings.ToLower(r.Host)
	if origin := r.Header.Get("Origin"); origin != "" && origin != "null" {
		return hostMatches(origin, expectedHost, trusted)
	}
	if ref := r.Header.Get("Referer"); ref != "" {
		return hostMatches(ref, expectedHost, trusted)
	}

	return false
}

func hostMatches(raw, expectedHost string, trusted map[string]struct{}) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}

	candidate := strings.ToLower(u.Scheme + "://" + u.Host)
	if _, ok := trusted[candidate]; ok {
		return true
	}

	return strings.EqualFold(u.Host, expectedHost)
}
