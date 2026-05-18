package httpx

import (
	"errors"
	"fmt"
	"net/http"
)

func ParseForm(w http.ResponseWriter, r *http.Request, maxBytes int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseForm(); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return fmt.Errorf("httpx.ParseForm: body too large: %w", err)
		}
		return fmt.Errorf("httpx.ParseForm: %w", err)
	}
	return nil
}
