package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/joho/godotenv"
)

type EnvConfig struct {
	Mode string `env:"MODE" default:"development" required:"true"`

	// MySQL or MariaDB
	DBHost          string `env:"DB_HOST"   default:"127.0.0.1"`
	DBPort          string `env:"DB_PORT" default:"3306"`
	DBUser          string `env:"DB_USER" required:"true"`
	DBPassword      string `env:"DB_PASSWORD" required:"true"`
	DBName          string `env:"DB_NAME" required:"true"`
	DBMinConnection int    `env:"DB_MIN_CONNECTIONS" default:"2"`
	DBMaxConnection int    `env:"DB_MAX_CONNECTIONS" default:"4"`
}

func ProcessEnv() (*EnvConfig, error) {
	// Try to find and load the env file
	// Don't catch error for docker declared env variables
	if envPath, err := getEnvFilePath(); err == nil {
		fmt.Println("Loading env file from ", envPath)
		_ = godotenv.Load(envPath)
	}

	env := &EnvConfig{}
	t := reflect.TypeOf(*env)
	v := reflect.ValueOf(env).Elem()

	for i := 0; i < t.NumField(); i++ {
		if err := processField(t.Field(i), v.Field(i)); err != nil {
			return nil, err
		}
	}

	return env, nil
}

func processField(field reflect.StructField, fieldVal reflect.Value) error {
	envTag := field.Tag.Get("env")
	if envTag == "" {
		return fmt.Errorf("env tag not found in field %s", field.Name)
	}

	value := os.Getenv(envTag)
	if value == "" {
		value = field.Tag.Get("default")
	}

	if field.Tag.Get("required") == "true" && value == "" {
		return fmt.Errorf("the value for the env field %s is required", envTag)
	}

	return setField(fieldVal, field.Type.Kind(), envTag, value)
}

func setField(fieldVal reflect.Value, kind reflect.Kind, envTag, value string) error {
	switch kind {
	case reflect.String:
		fieldVal.SetString(value)
	case reflect.Int:
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("the value for %s must be a valid integer", envTag)
		}
		fieldVal.SetInt(int64(intValue))
	default:
		return fmt.Errorf("unsupported field type %s for %s", kind, envTag)
	}
	return nil
}

func getEnvFilePath() (string, error) {
	dir, _ := os.Getwd()

	for {
		// Check for .env in the current directory
		envPath := filepath.Join(dir, ".env")
		if info, err := os.Stat(envPath); err == nil && !info.IsDir() {
			return envPath, nil
		}

		// Make sure we don't crawl outside the project if used in a mono-repo
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}

		// Move to the parent directory
		parentDir := filepath.Dir(dir)
		if parentDir == dir {
			break
		}
		dir = parentDir
	}

	return "", errors.New(".env file was not found")
}
