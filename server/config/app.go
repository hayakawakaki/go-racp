package config

import (
	"fmt"
	"os"
	"time"

	"github.com/goccy/go-yaml"
)

type GeneralConfig struct {
	ServerName string `yaml:"ServerName"`
}

type AppConfig struct {
	General    GeneralConfig `yaml:"GeneralConfig"`
	SessionTTL time.Duration `yaml:"SessionTTL"`
}

// appConfigDefaults apply default config in case of missing config file
func appConfigDefaults() *AppConfig {
	return &AppConfig{
		General:    GeneralConfig{ServerName: "Go Control Panel"},
		SessionTTL: 24 * time.Hour,
	}
}

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

	return cfg
}
