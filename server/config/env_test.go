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
				"MODE":               "production",
				"DB_HOST":            "10.0.0.1",
				"DB_PORT":            "3307",
				"DB_USER":            "testuser",
				"DB_PASSWORD":        "testpass",
				"DB_NAME":            "testdb",
				"DB_MIN_CONNECTIONS": "5",
				"DB_MAX_CONNECTIONS": "10",
			},
			expected: EnvConfig{
				Mode:            "production",
				DBHost:          "10.0.0.1",
				DBPort:          "3307",
				DBUser:          "testuser",
				DBPassword:      "testpass",
				DBName:          "testdb",
				DBMinConnection: 5,
				DBMaxConnection: 10,
			},
		},
		{
			name: "defaults applied",
			envVars: map[string]string{
				"DB_USER":     "testuser",
				"DB_PASSWORD": "testpass",
				"DB_NAME":     "testdb",
			},
			expected: EnvConfig{
				Mode:            "development",
				DBHost:          "127.0.0.1",
				DBPort:          "3306",
				DBUser:          "testuser",
				DBPassword:      "testpass",
				DBName:          "testdb",
				DBMinConnection: 2,
				DBMaxConnection: 4,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnvVars(t)

			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			got, err := ProcessEnv()
			if err != nil {
				t.Fatalf("ProcessEnv() unexpected error: %v", err)
			}

			if *got != tt.expected {
				t.Errorf("ProcessEnv() = %+v, want %+v", *got, tt.expected)
			}
		})
	}
}

func TestProcessEnv_Errors(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		expectedErr string
	}{
		{
			name:        "missing DB_USER",
			envVars:     map[string]string{},
			expectedErr: "the value for the env field DB_USER is required",
		},
		{
			name: "missing DB_PASSWORD",
			envVars: map[string]string{
				"DB_USER": "testuser",
			},
			expectedErr: "the value for the env field DB_PASSWORD is required",
		},
		{
			name: "missing DB_NAME",
			envVars: map[string]string{
				"DB_USER":     "testuser",
				"DB_PASSWORD": "testpass",
			},
			expectedErr: "the value for the env field DB_NAME is required",
		},
		{
			name: "invalid int value",
			envVars: map[string]string{
				"DB_USER":            "testuser",
				"DB_PASSWORD":        "testpass",
				"DB_NAME":            "testdb",
				"DB_MIN_CONNECTIONS": "one",
			},
			expectedErr: "the value for DB_MIN_CONNECTIONS must be a valid integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnvVars(t)

			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			_, err := ProcessEnv()
			if err == nil {
				t.Fatal("ProcessEnv() expected error, got nil")
			}

			if err.Error() != tt.expectedErr {
				t.Errorf("ProcessEnv() error = %q, want %q", err.Error(), tt.expectedErr)
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
