package httpx

import "net/http"

// IsHTMX reports whether the request was issued by htmx
func IsHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func Redirect(w http.ResponseWriter, r *http.Request, target string) {
	if IsHTMX(r) {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	http.Redirect(w, r, target, http.StatusSeeOther)
}
