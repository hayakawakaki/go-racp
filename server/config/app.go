package config

import (
	"fmt"
	"os"
	"time"

	"github.com/goccy/go-yaml"
)

// GeneralConfig holds UI/branding settings shared across every page.
type GeneralConfig struct {
	ServerName string `yaml:"ServerName"`
}

// MailerConfig holds outgoing-mail settings.
type MailerConfig struct {
	FromAddress string `yaml:"FromAddress"`
}

// AppConfig holds operator-tunable application settings loaded from config.yml.
type AppConfig struct {
	General              GeneralConfig `yaml:"GeneralConfig"`
	Mailer               MailerConfig  `yaml:"MailerConfig"`
	SessionTTL           time.Duration `yaml:"SessionTTL"`
	VerificationTokenTTL time.Duration `yaml:"VerificationTokenTTL"`
}

// appConfigDefaults apply default config in case of missing config file
func appConfigDefaults() *AppConfig {
	return &AppConfig{
		General:              GeneralConfig{ServerName: "Go Control Panel"},
		Mailer:               MailerConfig{FromAddress: "noreply@gocp.com"},
		SessionTTL:           24 * time.Hour,
		VerificationTokenTTL: 30 * time.Minute,
	}
}

// ProcessAppConfig loads config.yml from the project root, applying defaults for missing keys.
func ProcessAppConfig() *AppConfig {
	cfgPath, err := GetTargetFilePath("config.yml")
	if err != nil {
		panic(fmt.Errorf("missing config.yml: %w", err))
	}

	cfg := appConfigDefaults()
	//nolint:gosec // G304: cfgPath comes from GetTargetFilePath which walks the project tree from os.Getwd
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		panic(err)
	}

	// skip unmarshal on empty input
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			panic(err)
		}
	}

	if cfg.SessionTTL <= 0 {
		panic(fmt.Errorf("SessionTTL must be > 0, got %v", cfg.SessionTTL))
	}
	if cfg.VerificationTokenTTL <= 0 {
		panic(fmt.Errorf("VerificationTokenTTL must be > 0, got %v", cfg.VerificationTokenTTL))
	}
	if cfg.Mailer.FromAddress == "" {
		panic(fmt.Errorf("MailerConfig.FromAddress is required"))
	}

	return cfg
}
