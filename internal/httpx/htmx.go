package httpx

import "net/http"

// IsHTMX reports whether the request originated from an HTMX interaction by checking the HX-Request header.
// It returns true when the HX-Request header value equals "true".
func IsHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
