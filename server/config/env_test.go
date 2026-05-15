package config

import (
	"reflect"
	"testing"
)

func TestProcessEnv(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected EnvConfig
	}{
		{
			name: "all set",
			envVars: map[string]string{
				"MODE":                "production",
				"APP_URL":             "https://panel.example.com",
				"DB_MAIN_URL":         "user:pass@tcp(10.0.0.1:3306)/main",
				"DB_LOG_URL":          "user:pass@tcp(10.0.0.1:3306)/log",
				"DB_MAX_OPEN_CONN":    "10",
				"DB_MAX_IDLE_CONN":    "5",
				"DB_CP_URL":           "postgres://user:pass@10.0.0.1:5432/cp?sslmode=disable",
				"DB_CP_MAX_OPEN_CONN": "10",
				"DB_CP_MAX_IDLE_CONN": "5",
				"APP_PORT":            "9090",
				"SMTP_HOST":           "smtp.example.com",
				"SMTP_PORT":           "2525",
			},
			expected: EnvConfig{
				Mode:            "production",
				AppURL:          "https://panel.example.com",
				DBMainURL:       "user:pass@tcp(10.0.0.1:3306)/main",
				DBLogURL:        "user:pass@tcp(10.0.0.1:3306)/log",
				DBMaxOpenConn:   10,
				DBMaxIdleConn:   5,
				DBCPURL:         "postgres://user:pass@10.0.0.1:5432/cp?sslmode=disable",
				DBCPMaxOpenConn: 10,
				DBCPMaxIdleConn: 5,
				AppPort:         9090,
				SMTPHost:        "smtp.example.com",
				SMTPPort:        2525,
			},
		},
		{
			name: "defaults applied",
			envVars: map[string]string{
				"APP_URL":     "http://localhost:8080",
				"DB_MAIN_URL": "user:pass@tcp(127.0.0.1:3306)/main",
				"DB_LOG_URL":  "user:pass@tcp(127.0.0.1:3306)/log",
				"DB_CP_URL":   "postgres://dbuser:dbpass@db:5432/cp?sslmode=disable",
				"SMTP_HOST":   "mailpit",
			},
			expected: EnvConfig{
				Mode:            "development",
				AppURL:          "http://localhost:8080",
				DBMainURL:       "user:pass@tcp(127.0.0.1:3306)/main",
				DBLogURL:        "user:pass@tcp(127.0.0.1:3306)/log",
				DBMaxOpenConn:   4,
				DBMaxIdleConn:   4,
				DBCPURL:         "postgres://dbuser:dbpass@db:5432/cp?sslmode=disable",
				DBCPMaxOpenConn: 8,
				DBCPMaxIdleConn: 4,
				AppPort:         8080,
				SMTPHost:        "mailpit",
				SMTPPort:        587,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnvVars(t)

			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			got := ProcessEnv()

			if *got != tt.expected {
				t.Errorf("ProcessEnv() = %+v, want %+v", *got, tt.expected)
			}
		})
	}
}

// ProcessEnv calls log.Fatal on failure, which would kill the test process.
// Exercise the underlying processField directly to cover error paths.
func TestProcessField_Errors(t *testing.T) {
	tests := []struct {
		name        string
		envName     string
		envValue    string
		expectedErr string
	}{
		{
			name:        "missing required DB_MAIN_URL",
			envName:     "DB_MAIN_URL",
			envValue:    "",
			expectedErr: "the value for the env field DB_MAIN_URL is required",
		},
		{
			name:        "missing required DB_LOG_URL",
			envName:     "DB_LOG_URL",
			envValue:    "",
			expectedErr: "the value for the env field DB_LOG_URL is required",
		},
		{
			name:        "missing required SMTP_HOST",
			envName:     "SMTP_HOST",
			envValue:    "",
			expectedErr: "the value for the env field SMTP_HOST is required",
		},
		{
			name:        "invalid int DB_MAX_OPEN_CONN",
			envName:     "DB_MAX_OPEN_CONN",
			envValue:    "not-a-number",
			expectedErr: "the value for DB_MAX_OPEN_CONN must be a valid integer",
		},
		{
			name:        "invalid int DB_MAX_IDLE_CONN",
			envName:     "DB_MAX_IDLE_CONN",
			envValue:    "one",
			expectedErr: "the value for DB_MAX_IDLE_CONN must be a valid integer",
		},
		{
			name:        "missing required DB_CP_URL",
			envName:     "DB_CP_URL",
			envValue:    "",
			expectedErr: "the value for the env field DB_CP_URL is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnvVars(t)
			t.Setenv(tt.envName, tt.envValue)

			cfg := &EnvConfig{}
			v := reflect.ValueOf(cfg).Elem()

			var gotErr error
			found := false
			for field, fieldVal := range v.Fields() {
				if field.Tag.Get("env") == tt.envName {
					gotErr = processField(field, fieldVal)
					found = true
					break
				}
			}

			if !found {
				t.Fatalf("no field with env tag %q on EnvConfig", tt.envName)
			}

			if gotErr == nil {
				t.Fatal("processField() expected error, got nil")
			}

			if gotErr.Error() != tt.expectedErr {
				t.Errorf("processField() error = %q, want %q", gotErr.Error(), tt.expectedErr)
			}
		})
	}
}

func clearEnvVars(t *testing.T) {
	t.Helper()
	rt := reflect.TypeFor[EnvConfig]()
	for field := range rt.Fields() {
		if key := field.Tag.Get("env"); key != "" {
			t.Setenv(key, "")
		}
	}
}
