package domain

import "testing"

func TestFieldErrors_HasAndAdd(t *testing.T) {
	t.Parallel()

	fields := FieldErrors{}
	if fields.Has() {
		t.Errorf("Has() = true on empty FieldErrors, want false")
	}

	fields.Add("name", "name is required")
	if !fields.Has() {
		t.Errorf("Has() = false after Add, want true")
	}
	if got := fields["name"]; got != "name is required" {
		t.Errorf("fields[name] = %q, want %q", got, "name is required")
	}
}

func TestValidationError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		fields FieldErrors
		want   string
	}{
		{
			name:   "no fields",
			fields: FieldErrors{},
			want:   "validation failed",
		},
		{
			name:   "single field",
			fields: FieldErrors{"name": "name is required"},
			want:   "validation failed: name: name is required",
		},
		{
			name:   "fields rendered in sorted key order",
			fields: FieldErrors{"tier": "unknown rate tier", "name": "name is required"},
			want:   "validation failed: name: name is required; tier: unknown rate tier",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := &ValidationError{Fields: tt.fields}
			if got := err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}
