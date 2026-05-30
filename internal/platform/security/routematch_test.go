package security

import (
	"testing"
)

func TestRouteMatcher_Matches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		requestPath string
		routes      []string
		want        bool
	}{
		{
			name:        "exact route matches its own path",
			routes:      []string{"/account/password"},
			requestPath: "/account/password",
			want:        true,
		},
		{
			name:        "exact route does not match a different path",
			routes:      []string{"/account/password"},
			requestPath: "/account/email",
			want:        false,
		},
		{
			name:        "wildcard matches single trailing segment",
			routes:      []string{"/webhooks/*"},
			requestPath: "/webhooks/stripe",
			want:        true,
		},
		{
			name:        "wildcard matches nested trailing segments",
			routes:      []string{"/webhooks/*"},
			requestPath: "/webhooks/stripe/sub/path",
			want:        true,
		},
		{
			name:        "wildcard does not match without trailing segment",
			routes:      []string{"/webhooks/*"},
			requestPath: "/webhooks",
			want:        false,
		},
		{
			name:        "wildcard does not match unrelated path",
			routes:      []string{"/webhooks/*"},
			requestPath: "/account/password",
			want:        false,
		},
		{
			name:        "traversal does not escape the wildcard prefix",
			routes:      []string{"/webhooks/*"},
			requestPath: "/webhooks/../account/password",
			want:        false,
		},
		{
			name:        "dotted variant cleans and matches the wildcard",
			routes:      []string{"/webhooks/*"},
			requestPath: "/webhooks/./stripe",
			want:        true,
		},
		{
			name:        "non-canonical exact route matches the canonical path",
			routes:      []string{"/foo/"},
			requestPath: "/foo",
			want:        true,
		},
		{
			name:        "non-canonical exact route matches the trailing slash path",
			routes:      []string{"/foo/"},
			requestPath: "/foo/",
			want:        true,
		},
		{
			name:        "empty matcher matches nothing",
			routes:      nil,
			requestPath: "/anything",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m, err := NewRouteMatcher(tt.routes)
			if err != nil {
				t.Fatalf("NewRouteMatcher() error = %v, want nil", err)
			}
			if got := m.Matches(tt.requestPath); got != tt.want {
				t.Errorf("Matches(%q) = %v, want %v", tt.requestPath, got, tt.want)
			}
		})
	}
}

func TestNewRouteMatcher_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		routes  []string
		wantErr bool
	}{
		{
			name:    "valid wildcard route",
			routes:  []string{"/webhooks/*"},
			wantErr: false,
		},
		{
			name:    "valid exact routes",
			routes:  []string{"/account/password", "/foo/"},
			wantErr: false,
		},
		{
			name:    "route without leading slash",
			routes:  []string{"webhooks/*"},
			wantErr: true,
		},
		{
			name:    "bare wildcard",
			routes:  []string{"*"},
			wantErr: true,
		},
		{
			name:    "root wildcard",
			routes:  []string{"/*"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m, err := NewRouteMatcher(tt.routes)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewRouteMatcher() error = nil, want non-nil")
				}
				if m != nil {
					t.Errorf("NewRouteMatcher() matcher = %v, want nil on error", m)
				}

				return
			}
			if err != nil {
				t.Fatalf("NewRouteMatcher() error = %v, want nil", err)
			}
			if m == nil {
				t.Errorf("NewRouteMatcher() matcher = nil, want non-nil")
			}
		})
	}
}
