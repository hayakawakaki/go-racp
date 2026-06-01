package config

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"

	gomysql "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

//nolint:govet // field order is intentionally grouped by domain (app, db, mailer) over memory-optimal layout
type EnvConfig struct {
	// App Setting
	Mode    string `env:"MODE" default:"development"`
	AppPort int    `env:"APP_PORT" default:"8080"`
	AppURL  string `env:"APP_URL" required:"true"`

	// MySQL or MariaDB. Populated by Docker Compose.
	DBMainURL       string `env:"DB_MAIN_URL" required:"true"`
	DBLogURL        string `env:"DB_LOG_URL" required:"true"`
	DBMaxOpenConn   int    `env:"DB_MAX_OPEN_CONN" default:"4"`
	DBMaxIdleConn   int    `env:"DB_MAX_IDLE_CONN" default:"4"`
	DBCPURL         string `env:"DB_CP_URL" required:"true"`
	DBCPMaxOpenConn int    `env:"DB_CP_MAX_OPEN_CONN" default:"8"`
	DBCPMaxIdleConn int    `env:"DB_CP_MAX_IDLE_CONN" default:"4"`

	// Mailer
	SMTPHost string `env:"SMTP_HOST" required:"true"`
	SMTPPort int    `env:"SMTP_PORT" default:"587"`

	// Security
	CSRFSecret string `env:"CSRF_SECRET" required:"true"`

	// Payment platforms
	StripeSecretKey      string `env:"STRIPE_SECRET_KEY" default:""`
	StripeWebhookSecret  string `env:"STRIPE_WEBHOOK_SECRET" default:""`
	PaypalClientID       string `env:"PAYPAL_CLIENT_ID" default:""`
	PaypalSecret         string `env:"PAYPAL_SECRET" default:""`
	PaypalWebhookID      string `env:"PAYPAL_WEBHOOK_ID" default:""`
	NowpaymentsAPIKey    string `env:"NOWPAYMENTS_API_KEY" default:""`
	NowpaymentsIPNSecret string `env:"NOWPAYMENTS_IPN_SECRET" default:""`

	// Server Status
	ServerHost string `env:"SERVER_HOST" default:"127.0.0.1"`
	LoginPort  int    `env:"LOGIN_PORT" default:"6900"`
	CharPort   int    `env:"CHAR_PORT" default:"6121"`
	MapPort    int    `env:"MAP_PORT" default:"5121"`
	WebPort    int    `env:"WEB_PORT" default:"8888"`
}

// ProcessEnv loads the project .env file when present and populates an EnvConfig from environment variables, terminating the process via log.Fatal on a missing required value or type mismatch.
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

	if err := env.validate(); err != nil {
		log.Fatal(err)
	}

	return env
}

func (env *EnvConfig) validate() error {
	if env.Mode != "production" {
		return nil
	}

	dsns := []struct {
		envName string
		url     string
	}{
		{"DB_MAIN_URL", env.DBMainURL},
		{"DB_LOG_URL", env.DBLogURL},
	}

	for _, dsn := range dsns {
		if err := guardMariaDBRoot(dsn.envName, dsn.url); err != nil {
			return err
		}
	}

	return nil
}

func guardMariaDBRoot(envName, url string) error {
	cfg, err := gomysql.ParseDSN(url)
	if err != nil {
		return fmt.Errorf("the value for %s is not a valid MariaDB DSN: %w", envName, err)
	}

	if strings.EqualFold(cfg.User, "root") {
		return fmt.Errorf("the MariaDB user for %s must not be root in production", envName)
	}

	return nil
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
