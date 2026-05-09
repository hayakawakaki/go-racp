package config

import (
	"os"
	"time"

	"github.com/goccy/go-yaml"
)

type AppConfig struct {
	ServerName string        `yaml:"ServerName" default:"Go Control Panel"`
	SessionTTL time.Duration `yaml:"SessionTTL" default:"24h"`
}

func ProcessAppConfig() *AppConfig {
	cfgPath, err := GetTargetFilePath("config.yml")
	if err != nil {
		panic("missing config.yml")
	}

	cfg := &AppConfig{}
	//nolint:gosec // G304: cfgPath comes from GetTargetFilePath which walks the project tree from os.Getwd
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		panic(err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		panic(err)
	}

	return cfg
}
