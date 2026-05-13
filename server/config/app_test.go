package config

import (
	"strings"
	"testing"
)

func mustPanic(t *testing.T, fn func()) string {
	t.Helper()
	var got any
	func() {
		defer func() { got = recover() }()
		fn()
	}()
	if got == nil {
		t.Fatalf("expected panic, got none")
	}
	switch value := got.(type) {
	case error:
		return value.Error()
	case string:
		return value
	default:
		t.Fatalf("unexpected panic type %T: %v", got, got)
		return ""
	}
}

func TestValidateGroupConfig_AcceptsDistinctPositiveNonAdminValues(t *testing.T) {
	t.Parallel()
	cfg := &GroupConfig{Moderator: 20, Enforcer: 10, Event: 2}
	validateGroupConfig(cfg)
}

func TestValidateGroupConfig_RejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		wantContain string
		cfg         GroupConfig
	}{
		{
			name:        "moderator zero",
			cfg:         GroupConfig{Moderator: 0, Enforcer: 10, Event: 2},
			wantContain: "Group.Moderator must be > 0",
		},
		{
			name:        "enforcer negative",
			cfg:         GroupConfig{Moderator: 20, Enforcer: -1, Event: 2},
			wantContain: "Group.Enforcer must be > 0",
		},
		{
			name:        "event zero",
			cfg:         GroupConfig{Moderator: 20, Enforcer: 10, Event: 0},
			wantContain: "Group.Event must be > 0",
		},
		{
			name:        "moderator reserved 99",
			cfg:         GroupConfig{Moderator: 99, Enforcer: 10, Event: 2},
			wantContain: "Group.Moderator = 99 is reserved for admin",
		},
		{
			name:        "enforcer reserved 99",
			cfg:         GroupConfig{Moderator: 20, Enforcer: 99, Event: 2},
			wantContain: "Group.Enforcer = 99 is reserved for admin",
		},
		{
			name:        "event reserved 99",
			cfg:         GroupConfig{Moderator: 20, Enforcer: 10, Event: 99},
			wantContain: "Group.Event = 99 is reserved for admin",
		},
		{
			name:        "moderator equals enforcer",
			cfg:         GroupConfig{Moderator: 10, Enforcer: 10, Event: 2},
			wantContain: "must be distinct",
		},
		{
			name:        "moderator equals event",
			cfg:         GroupConfig{Moderator: 5, Enforcer: 10, Event: 5},
			wantContain: "must be distinct",
		},
		{
			name:        "enforcer equals event",
			cfg:         GroupConfig{Moderator: 20, Enforcer: 7, Event: 7},
			wantContain: "must be distinct",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := tt.cfg
			msg := mustPanic(t, func() { validateGroupConfig(&cfg) })
			if !strings.Contains(msg, tt.wantContain) {
				t.Errorf("panic message = %q, want substring %q", msg, tt.wantContain)
			}
		})
	}
}
