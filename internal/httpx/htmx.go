package httpx

import "net/http"

// IsHTMX reports whether the request originated from an htmx client by checking
// if the "HX-Request" header is exactly "true". It returns `true` when the
// header value equals "true", `false` otherwise.
func IsHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
