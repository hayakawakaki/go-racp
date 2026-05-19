package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateSubject(t *testing.T) {
	t.Parallel()

	longSubject := strings.Repeat("a", MaxSubjectLen+1)
	tests := []struct {
		wantErr error
		name    string
		in      string
		want    string
	}{
		{nil, "trims and accepts", "  hello  ", "hello"},
		{ErrSubjectEmpty, "empty", "", ""},
		{ErrSubjectEmpty, "only spaces", "   ", ""},
		{ErrSubjectTooLong, "too long", longSubject, ""},
	}
	for _, tt := range tests {
		got, err := ValidateSubject(tt.in)
		if !errors.Is(err, tt.wantErr) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestValidateBody(t *testing.T) {
	t.Parallel()

	longBody := strings.Repeat("x", MaxBodyLen+1)
	tests := []struct {
		wantErr error
		name    string
		in      string
		want    string
	}{
		{nil, "trims and accepts", "  hi  ", "hi"},
		{ErrBodyEmpty, "empty", "", ""},
		{ErrBodyTooLong, "too long", longBody, ""},
	}
	for _, tt := range tests {
		got, err := ValidateBody(tt.in)
		if !errors.Is(err, tt.wantErr) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
