package domain

import (
	"errors"
	"testing"
)

func TestFieldErrors_HasAndAdd(t *testing.T) {
	t.Parallel()

	fe := FieldErrors{}
	if fe.Has() {
		t.Errorf("empty FieldErrors should not Has()")
	}
	fe.Add("username", "too short")
	if !fe.Has() {
		t.Errorf("populated FieldErrors should Has()")
	}
	if got := fe["username"]; got != "too short" {
		t.Errorf("fe[username] = %q, want %q", got, "too short")
	}
}

func TestValidationError_ErrorAndAs(t *testing.T) {
	t.Parallel()

	ve := &ValidationError{Fields: FieldErrors{"email": "bad shape"}}

	if ve.Error() == "" {
		t.Errorf("ValidationError.Error() returned empty string")
	}

	var got *ValidationError
	if !errors.As(error(ve), &got) {
		t.Fatalf("errors.As failed to extract ValidationError")
	}
	if got.Fields["email"] != "bad shape" {
		t.Errorf("got.Fields[email] = %q, want %q", got.Fields["email"], "bad shape")
	}
}
