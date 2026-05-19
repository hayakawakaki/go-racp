package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		in      string
	}{
		{nil, "single char", "x"},
		{nil, "trims surrounding whitespace", "  hello  "},
		{nil, "at rune limit", strings.Repeat("a", MaxTitleLen)},
		{nil, "multibyte at rune limit", strings.Repeat("日", MaxTitleLen)},
		{ErrTitleEmpty, "empty", ""},
		{ErrTitleEmpty, "whitespace only", "   "},
		{ErrTitleTooLong, "over rune limit", strings.Repeat("a", MaxTitleLen+1)},
		{ErrTitleTooLong, "multibyte over rune limit", strings.Repeat("日", MaxTitleLen+1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateTitle(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateTitle err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		in      string
	}{
		{nil, "single char", "x"},
		{nil, "at byte limit after trim", "   " + strings.Repeat("x", MaxBodyLen) + "   "},
		{nil, "exactly at byte limit", strings.Repeat("x", MaxBodyLen)},
		{ErrBodyEmpty, "empty", ""},
		{ErrBodyEmpty, "whitespace only", "   "},
		{ErrBodyTooLong, "over byte limit", strings.Repeat("x", MaxBodyLen+1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateBody(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateBody err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
