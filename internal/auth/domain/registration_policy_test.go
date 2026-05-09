package domain

import "testing"

func TestCheckRegistrationUsername(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "ok 6 chars", input: "abcdef"},
		{name: "ok 23 chars", input: "abcdefghijklmnopqrstuvw"},
		{name: "too short 5", input: "abcde", wantErr: true},
		{name: "too short 1", input: "a", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := CheckRegistrationUsername(tc.input)
			if tc.wantErr && err == nil {
				t.Errorf("CheckRegistrationUsername(%q) = nil; want error", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("CheckRegistrationUsername(%q) = %v; want nil", tc.input, err)
			}
		})
	}
}

func TestCheckRegistrationPassword(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "ok min", input: "Abcd123!"},
		{name: "ok no lowercase needed", input: "PASS123!"},
		{name: "ok long", input: "MyTest1234!Password"},
		{name: "too short 7", input: "Ab12!XY", wantErr: true},
		{name: "no upper", input: "abcd123!", wantErr: true},
		{name: "no digit", input: "Abcdefg!", wantErr: true},
		{name: "no special", input: "Abcd1234", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := CheckRegistrationPassword(tc.input)
			if tc.wantErr && err == nil {
				t.Errorf("CheckRegistrationPassword(%q) = nil; want error", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("CheckRegistrationPassword(%q) = %v; want nil", tc.input, err)
			}
		})
	}
}
