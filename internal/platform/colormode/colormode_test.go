package colormode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsDark_DefaultsToDark(t *testing.T) {
	t.Parallel()

	if !IsDark(context.Background()) {
		t.Errorf("IsDark(empty context) = false, want true")
	}
}

func TestIsDark_NonStringValueDefaultsToDark(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), modeKey, 123)
	if !IsDark(ctx) {
		t.Errorf("IsDark(non-string value) = false, want true")
	}
}

func TestMiddleware_NormalizesCookieToMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cookie    string
		setCookie bool
		wantDark  bool
	}{
		{name: "no cookie defaults to dark", setCookie: false, wantDark: true},
		{name: "light cookie renders light", setCookie: true, cookie: "light", wantDark: false},
		{name: "dark cookie renders dark", setCookie: true, cookie: "dark", wantDark: true},
		{name: "empty value defaults to dark", setCookie: true, cookie: "", wantDark: true},
		{name: "capitalized Light is not light", setCookie: true, cookie: "Light", wantDark: true},
		{name: "uppercase LIGHT is not light", setCookie: true, cookie: "LIGHT", wantDark: true},
		{name: "injection payload defaults to dark", setCookie: true, cookie: "<script>alert(1)</script>", wantDark: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotDark bool
			called := 0
			next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				called++
				gotDark = IsDark(r.Context())
			})

			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			if tt.setCookie {
				req.AddCookie(&http.Cookie{Name: cookieName, Value: tt.cookie})
			}
			rr := httptest.NewRecorder()

			Middleware(next).ServeHTTP(rr, req)

			if called != 1 {
				t.Errorf("next called %d times, want 1", called)
			}
			if gotDark != tt.wantDark {
				t.Errorf("IsDark = %v, want %v", gotDark, tt.wantDark)
			}
			if cookies := rr.Result().Cookies(); len(cookies) != 0 {
				t.Errorf("middleware set %d cookies, want 0", len(cookies))
			}
		})
	}
}
