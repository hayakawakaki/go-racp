package httpx

import "net/http"

// IsHTMX reports whether the request was issued by htmx
func IsHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true" && !IsBoosted(r)
}

func IsBoosted(r *http.Request) bool {
	return r.Header.Get("HX-Boosted") == "true"
}

// Redirect issues an HX-Redirect response for htmx requests and a standard 303 redirect otherwise.
func Redirect(w http.ResponseWriter, r *http.Request, target string) {
	if IsHTMX(r) || IsBoosted(r) {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	http.Redirect(w, r, target, http.StatusSeeOther)
}
