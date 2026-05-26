package manifest

import (
	"strings"
	"testing"
)

const validManifest = `Name: default
DisplayName: "Default"
Version: "1.0"
Author:
  Name: "kaki"
Compatible:
  Min: "1.0"
`

func TestParseManifest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		check     func(t *testing.T, m Manifest)
		name      string
		input     string
		errSubstr string
		wantErr   bool
	}{
		{
			name:  "valid full manifest",
			input: validManifest,
			check: func(t *testing.T, m Manifest) {
				if m.Name != "default" {
					t.Errorf("Name = %q, want %q", m.Name, "default")
				}
				if m.DisplayName != "Default" {
					t.Errorf("DisplayName = %q, want %q", m.DisplayName, "Default")
				}
				if m.Version != "1.0" {
					t.Errorf("Version = %q, want %q", m.Version, "1.0")
				}
				if m.Author.Name != "kaki" {
					t.Errorf("Author.Name = %q, want %q", m.Author.Name, "kaki")
				}
				if m.Compatible.Min != "1.0" {
					t.Errorf("Compatible.Min = %q, want %q", m.Compatible.Min, "1.0")
				}
			},
		},
		{
			name: "with author URL",
			input: `Name: sample
DisplayName: "Sample"
Version: "1.2"
Author:
  Name: "kaki"
  Url: "https://example.com"
Compatible:
  Min: "1.0"
`,
			check: func(t *testing.T, m Manifest) {
				if m.Author.URL != "https://example.com" {
					t.Errorf("Author.URL = %q, want %q", m.Author.URL, "https://example.com")
				}
			},
		},
		{
			name: "name with underscores and digits",
			input: `Name: arcade_blue_v2
DisplayName: "Arcade Blue"
Version: "1.0"
Author: {Name: "k"}
Compatible: {Min: "1.0"}
`,
		},
		{
			name: "missing Name",
			input: `DisplayName: "X"
Version: "1.0"
Author: {Name: "k"}
Compatible: {Min: "1.0"}
`,
			wantErr:   true,
			errSubstr: "Name is required",
		},
		{
			name: "Name with spaces",
			input: `Name: "Default GoCP Theme"
DisplayName: "X"
Version: "1.0"
Author: {Name: "k"}
Compatible: {Min: "1.0"}
`,
			wantErr:   true,
			errSubstr: "must match",
		},
		{
			name: "Name with capitals",
			input: `Name: Default
DisplayName: "X"
Version: "1.0"
Author: {Name: "k"}
Compatible: {Min: "1.0"}
`,
			wantErr:   true,
			errSubstr: "must match",
		},
		{
			name: "Name with hyphen",
			input: `Name: my-theme
DisplayName: "X"
Version: "1.0"
Author: {Name: "k"}
Compatible: {Min: "1.0"}
`,
			wantErr:   true,
			errSubstr: "must match",
		},
		{
			name: "missing DisplayName",
			input: `Name: default
Version: "1.0"
Author: {Name: "k"}
Compatible: {Min: "1.0"}
`,
			wantErr:   true,
			errSubstr: "DisplayName is required",
		},
		{
			name: "Version missing minor",
			input: `Name: default
DisplayName: "X"
Version: "1"
Author: {Name: "k"}
Compatible: {Min: "1.0"}
`,
			wantErr:   true,
			errSubstr: "Version",
		},
		{
			name: "Version with three segments",
			input: `Name: default
DisplayName: "X"
Version: "1.0.0"
Author: {Name: "k"}
Compatible: {Min: "1.0"}
`,
			wantErr:   true,
			errSubstr: "Version",
		},
		{
			name: "Version non-numeric",
			input: `Name: default
DisplayName: "X"
Version: "v1.0"
Author: {Name: "k"}
Compatible: {Min: "1.0"}
`,
			wantErr:   true,
			errSubstr: "Version",
		},
		{
			name: "missing Author Name",
			input: `Name: default
DisplayName: "X"
Version: "1.0"
Author: {}
Compatible: {Min: "1.0"}
`,
			wantErr:   true,
			errSubstr: "Author.Name is required",
		},
		{
			name: "missing Compatible Min",
			input: `Name: default
DisplayName: "X"
Version: "1.0"
Author: {Name: "k"}
Compatible: {}
`,
			wantErr:   true,
			errSubstr: "Compatible.Min",
		},
		{
			name: "Compatible Min wildcard rejected",
			input: `Name: default
DisplayName: "X"
Version: "1.0"
Author: {Name: "k"}
Compatible: {Min: "1.x"}
`,
			wantErr:   true,
			errSubstr: "Compatible.Min",
		},
		{
			name:      "invalid yaml syntax",
			input:     `Name: [unclosed`,
			wantErr:   true,
			errSubstr: "parse manifest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m, err := ParseManifest([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil; manifest=%+v", m)
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.check != nil {
				tt.check(t, m)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantMaj int
		wantMin int
		wantErr bool
	}{
		{name: "zero", input: "0.0", wantMaj: 0, wantMin: 0},
		{name: "one zero", input: "1.0", wantMaj: 1, wantMin: 0},
		{name: "one one", input: "1.1", wantMaj: 1, wantMin: 1},
		{name: "double digit minor", input: "1.42", wantMaj: 1, wantMin: 42},
		{name: "double digit major", input: "12.0", wantMaj: 12, wantMin: 0},
		{name: "both double digit", input: "12.34", wantMaj: 12, wantMin: 34},
		{name: "empty", input: "", wantErr: true},
		{name: "single segment", input: "1", wantErr: true},
		{name: "three segments", input: "1.0.0", wantErr: true},
		{name: "v prefix", input: "v1.0", wantErr: true},
		{name: "minor wildcard", input: "1.x", wantErr: true},
		{name: "major wildcard", input: "x.0", wantErr: true},
		{name: "trailing dot", input: "1.", wantErr: true},
		{name: "leading dot", input: ".0", wantErr: true},
		{name: "negative major", input: "-1.0", wantErr: true},
		{name: "non-numeric", input: "abc", wantErr: true},
		{name: "trailing space", input: "1.0 ", wantErr: true},
		{name: "leading space", input: " 1.0", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			major, minor, err := ParseVersion(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil (major=%d minor=%d)", tt.input, major, minor)
				}

				return
			}

			if err != nil {
				t.Errorf("unexpected error for %q: %v", tt.input, err)

				return
			}

			if major != tt.wantMaj {
				t.Errorf("major = %d, want %d", major, tt.wantMaj)
			}

			if minor != tt.wantMin {
				t.Errorf("minor = %d, want %d", minor, tt.wantMin)
			}
		})
	}
}

func TestVersionAtLeast(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		actual  string
		minimum string
		want    bool
		wantErr bool
	}{
		{name: "equal versions", actual: "1.0", minimum: "1.0", want: true},
		{name: "actual newer minor", actual: "1.5", minimum: "1.0", want: true},
		{name: "actual newer major", actual: "2.0", minimum: "1.0", want: true},
		{name: "actual newer minor than minor", actual: "1.5", minimum: "1.4", want: true},
		{name: "actual older minor", actual: "1.0", minimum: "1.1", want: false},
		{name: "actual older major", actual: "1.0", minimum: "2.0", want: false},
		{name: "newer major wins despite older minor", actual: "2.0", minimum: "1.999", want: true},
		{name: "older major loses despite newer minor", actual: "1.999", minimum: "2.0", want: false},
		{name: "zero zero baseline", actual: "0.0", minimum: "0.0", want: true},
		{name: "zero zero vs one zero", actual: "0.0", minimum: "1.0", want: false},
		{name: "invalid actual", actual: "x", minimum: "1.0", wantErr: true},
		{name: "invalid minimum", actual: "1.0", minimum: "x", wantErr: true},
		{name: "empty actual", actual: "", minimum: "1.0", wantErr: true},
		{name: "empty minimum", actual: "1.0", minimum: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := VersionAtLeast(tt.actual, tt.minimum)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)

				return
			}

			if got != tt.want {
				t.Errorf("VersionAtLeast(%q, %q) = %v, want %v", tt.actual, tt.minimum, got, tt.want)
			}
		})
	}
}
