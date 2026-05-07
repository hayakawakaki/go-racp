package httpx

import "net/http"

func IsHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
