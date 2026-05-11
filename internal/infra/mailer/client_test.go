package mailer

import (
	"errors"
	"testing"

	"github.com/wneessen/go-mail"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expectedErr  error
		name         string
		host         string
		expectedAddr string
		port         int
	}{
		{
			name:         "valid host and port",
			host:         "smtp.example.com",
			port:         2525,
			expectedAddr: "smtp.example.com:2525",
		},
		{
			name:         "submission port",
			host:         "mailpit",
			port:         587,
			expectedAddr: "mailpit:587",
		},
		{
			name:        "empty host",
			host:        "",
			port:        587,
			expectedErr: mail.ErrNoHostname,
		},
		{
			name:        "port below valid range",
			host:        "smtp.example.com",
			port:        0,
			expectedErr: mail.ErrInvalidPort,
		},
		{
			name:        "port above valid range",
			host:        "smtp.example.com",
			port:        70000,
			expectedErr: mail.ErrInvalidPort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewClient(tt.host, tt.port, false)

			if tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Fatalf("NewClient() error = %v, want wrapped %v", err, tt.expectedErr)
				}
				if got != nil {
					t.Errorf("NewClient() client = %v, want nil on error", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewClient() unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("NewClient() client = nil, want non-nil")
			}
			if addr := got.ServerAddr(); addr != tt.expectedAddr {
				t.Errorf("ServerAddr() = %q, want %q", addr, tt.expectedAddr)
			}
		})
	}
}
