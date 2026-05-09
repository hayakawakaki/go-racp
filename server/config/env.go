package config

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"

	"github.com/joho/godotenv"
)

type EnvConfig struct {
	Mode string `env:"MODE" default:"development"`

	// MySQL or MariaDB. Populated by Docker Compose.
	DBMainURL     string `env:"DB_MAIN_URL" required:"true"`
	DBLogURL      string `env:"DB_LOG_URL" required:"true"`
	DBMaxOpenConn int    `env:"DB_MAX_OPEN_CONN" default:"4"`
	DBMaxIdleConn int    `env:"DB_MAX_IDLE_CONN" default:"4"`

	// App Setting
	AppPort int `env:"APP_PORT" default:"8080"`
}

func ProcessEnv() *EnvConfig {
	// Try to find and load the env file
	// Don't catch error for docker declared env variables
	if envPath, err := GetTargetFilePath(".env"); err == nil {
		fmt.Println("Loading env file from ", envPath)
		_ = godotenv.Load(envPath)
	}

	env := &EnvConfig{}
	v := reflect.ValueOf(env).Elem()

	for field, fieldVal := range v.Fields() {
		if err := processField(field, fieldVal); err != nil {
			log.Fatal(err)
		}
	}

	return env
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
