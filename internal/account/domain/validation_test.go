package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidateUsername(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "ok lower", input: "abcde"},
		{name: "ok upper", input: "USER123"},
		{name: "ok digits", input: "user123"},
		{name: "ok underscore", input: "a_b_c"},
		{name: "ok min length 1", input: "a"},
		{name: "ok max length 23", input: "abcdefghijklmnopqrstuvw"},
		{name: "empty", input: "", wantErr: true},
		{name: "too long 24", input: "abcdefghijklmnopqrstuvwx", wantErr: true},
		{name: "space", input: "crazy arashi", wantErr: true},
		{name: "dash", input: "crazy-arashi", wantErr: true},
		{name: "dot", input: "crazy.arashi", wantErr: true},
		{name: "unicode", input: "crázyarashi", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateUsername(tc.input)
			if tc.wantErr && err == nil {
				t.Errorf("ValidateUsername(%q) = nil; want error", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ValidateUsername(%q) = %v; want nil", tc.input, err)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	t.Parallel()

	exact39 := "a@" + strings.Repeat("b", 37)
	exact40 := "a@" + strings.Repeat("b", 38)

	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "ok simple", input: "a@x.io", want: "a@x.io"},
		{name: "ok lowercased", input: "Test@Example.COM", want: "test@example.com"},
		{name: "ok max length 39", input: exact39, want: exact39},
		{name: "empty", input: "", wantErr: true},
		{name: "no at", input: "notmail", wantErr: true},
		{name: "garbage", input: "not an email", wantErr: true},
		{name: "too long 40", input: exact40, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ValidateEmail(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("ValidateEmail(%q) = %q, nil; want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateEmail(%q) error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("got = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "ok 1 char", input: "x"},
		{name: "ok 32 chars", input: "abcdefghijklmnopqrstuvwxyz123456"},
		{name: "ok mixed", input: "Test1234!"},
		{name: "empty", input: "", wantErr: true},
		{name: "too long 33", input: "abcdefghijklmnopqrstuvwxyz1234567", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePassword(tc.input)
			if tc.wantErr && err == nil {
				t.Errorf("ValidatePassword(%q) = nil; want error", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ValidatePassword(%q) = %v; want nil", tc.input, err)
			}
		})
	}
}

func TestValidateGender(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "ok M", input: "M"},
		{name: "ok F", input: "F"},
		{name: "lower m rejected", input: "m", wantErr: true},
		{name: "S reserved", input: "S", wantErr: true},
		{name: "empty", input: "", wantErr: true},
		{name: "garbage", input: "X", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateGender(tc.input)
			if tc.wantErr && err == nil {
				t.Errorf("ValidateGender(%q) = nil; want error", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ValidateGender(%q) = %v; want nil", tc.input, err)
			}
		})
	}
}

func TestValidateBirthdate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		wantErr error
		name    string
		input   string
	}{
		{name: "ok common", input: "2000-01-01"},
		{name: "ok today", input: "2026-05-12"},
		{name: "ok exactly 120y ago", input: "1906-05-12"},
		{name: "empty", input: "", wantErr: ErrBirthdateEmpty},
		{name: "not a date", input: "not-a-date", wantErr: ErrBirthdateShape},
		{name: "month 13", input: "2026-13-01", wantErr: ErrBirthdateShape},
		{name: "wrong format slashes", input: "2000/01/01", wantErr: ErrBirthdateShape},
		{name: "future tomorrow", input: "2026-05-13", wantErr: ErrBirthdateFuture},
		{name: "older than 120y", input: "1905-05-11", wantErr: ErrBirthdateTooOld},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ValidateBirthdate(tc.input, now)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("ValidateBirthdate(%q) err = %v; want %v", tc.input, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateBirthdate(%q) = %v; want nil", tc.input, err)
			}
			want, parseErr := time.Parse("2006-01-02", tc.input)
			if parseErr != nil {
				t.Fatalf("test data %q failed to parse: %v", tc.input, parseErr)
			}
			if !got.Equal(want) {
				t.Errorf("ValidateBirthdate(%q) = %v; want %v", tc.input, got, want)
			}
		})
	}
}
