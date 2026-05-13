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

type RolesConfig map[string]ActionRoles

var allowedRoleStrings = map[string]struct{}{
	"*":         {},
	"Player":    {},
	"Event":     {},
	"Moderator": {},
	"Enforcer":  {},
}

func ProcessRolesConfig() RolesConfig {
	cfgPath, err := GetTargetFilePath("roles.yml")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			cfg := RolesConfig{}
			validateRolesConfig(cfg)

			return cfg
		}
		panic(fmt.Errorf("locate roles.yml: %w", err))
	}
	cfg, err := loadRolesConfig(cfgPath)
	if err != nil {
		panic(err)
	}
	validateRolesConfig(cfg)

	return cfg
}

func loadRolesConfig(path string) (RolesConfig, error) {
	//nolint:gosec // G304: path comes from GetTargetFilePath which walks the project tree.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read roles.yml: %w", err)
	}

	return parseRolesConfig(data)
}

func parseRolesConfig(data []byte) (RolesConfig, error) {
	cfg := RolesConfig{}
	if len(data) == 0 {
		return cfg, nil
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse roles.yml: %w", err)
	}

	return cfg, nil
}

func validateRolesConfig(cfg RolesConfig) {
	if _, hasAdmin := cfg["Admin"]; hasAdmin {
		panic(fmt.Errorf("roles.yml: top-level 'Admin' key is forbidden — Admin is hardcoded"))
	}
	for groupName, actions := range cfg {
		for actionName, list := range actions {
			fullName := groupName + "." + actionName
			if list == nil {
				continue
			}
			if len(list) == 0 {
				panic(fmt.Errorf("roles.yml: Action '%s' has an empty list — would deny everyone; use a non-empty list or remove the entry", fullName))
			}
			for _, role := range list {
				if role == "Admin" {
					panic(fmt.Errorf("roles.yml: Action '%s' lists 'Admin' — Admin is implicit, remove it from the list", fullName))
				}
				if _, ok := allowedRoleStrings[role]; !ok {
					panic(fmt.Errorf("roles.yml: Action '%s' references unknown role '%s'", fullName, role))
				}
			}
		}
	}
}
