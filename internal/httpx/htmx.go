package httpx

import "net/http"

// IsHTMX reports whether the request was issued by htmx
func IsHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
