package config

import (
	"os"
	"time"

	"github.com/goccy/go-yaml"
)

type AppConfig struct {
	ServerName string        `yaml:"ServerName"`
	SessionTTL time.Duration `yaml:"SessionTTL"`
}

// appConfigDefaults apply default config in case of missing config file
func appConfigDefaults() *AppConfig {
	return &AppConfig{
		ServerName: "Go Control Panel",
		SessionTTL: 24 * time.Hour,
	}
}

func ProcessAppConfig() *AppConfig {
	cfgPath, err := GetTargetFilePath("config.yml")
	if err != nil {
		panic("missing config.yml")
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

	return cfg
}
