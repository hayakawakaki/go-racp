package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/goccy/go-yaml"
)

type RoleList []string

type ActionRoles map[string]RoleList

type AccessConfig map[string]ActionRoles

func ProcessAccessConfig() AccessConfig {
	cfgPath, err := GetTargetFilePath("access.yml")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			cfg := AccessConfig{}
			validateAccessConfig(cfg)

			return cfg
		}
		panic(fmt.Errorf("locate access.yml: %w", err))
	}
	cfg, err := loadAccessConfig(cfgPath)
	if err != nil {
		panic(err)
	}
	validateAccessConfig(cfg)

	return cfg
}

func loadAccessConfig(path string) (AccessConfig, error) {
	//nolint:gosec // G304: path comes from GetTargetFilePath which walks the project tree.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read access.yml: %w", err)
	}

	return parseAccessConfig(data)
}

func parseAccessConfig(data []byte) (AccessConfig, error) {
	cfg := AccessConfig{}
	if len(data) == 0 {
		return cfg, nil
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse access.yml: %w", err)
	}

	return cfg, nil
}

func validateAccessConfig(cfg AccessConfig) {
	if _, hasAdmin := cfg["Admin"]; hasAdmin {
		panic(fmt.Errorf("access.yml: top-level 'Admin' key is forbidden — Admin is hardcoded"))
	}
	for groupName, actions := range cfg {
		for actionName, list := range actions {
			fullName := groupName + "." + actionName
			if list == nil {
				continue
			}
			if len(list) == 0 {
				panic(fmt.Errorf("access.yml: Action '%s' has an empty list — would deny everyone; use a non-empty list or remove the entry", fullName))
			}
			for _, role := range list {
				if role == "Admin" {
					panic(fmt.Errorf("access.yml: Action '%s' lists 'Admin' — Admin is implicit, remove it from the list", fullName))
				}
				if role == "" {
					panic(fmt.Errorf("access.yml: Action '%s' has an empty role name", fullName))
				}
			}
		}
	}
}
