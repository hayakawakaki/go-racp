package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"

	"github.com/goccy/go-yaml"
)

type RoleList []string

type Entry struct {
	Roles    RoleList
	Requires []string
}

type ActionRoles map[string]Entry

type AccessConfig map[string]ActionRoles

func (e Entry) RequiresFullAccess() bool {
	return slices.Contains(e.Requires, "FullAccess")
}

func (e *Entry) UnmarshalYAML(unmarshal func(any) error) error {
	var asList RoleList
	if err := unmarshal(&asList); err == nil {
		e.Roles = asList
		e.Requires = nil
		return nil
	}

	var asStruct struct {
		Roles    RoleList `yaml:"roles"`
		Requires []string `yaml:"requires"`
	}
	if err := unmarshal(&asStruct); err != nil {
		return fmt.Errorf("access.yml entry: expected list of roles or { roles, requires }: %w", err)
	}
	e.Roles = asStruct.Roles
	e.Requires = asStruct.Requires
	return nil
}

func (c AccessConfig) ManageRoles(group string) []string {
	actions, ok := c[group]
	if !ok {
		return nil
	}
	entry, ok := actions["Manage"]
	if !ok {
		return nil
	}

	return entry.Roles
}

// ProcessAccessConfig loads and validates access.yml from the project root, returning an empty config when the file is absent and panicking on any parse or schema error.
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

//nolint:cyclop // sequential per-entry validation, splitting would scatter the error messages
func validateAccessConfig(cfg AccessConfig) {
	knownTags := map[string]struct{}{"FullAccess": {}}

	if _, hasAdmin := cfg[adminRoleName]; hasAdmin {
		panic(fmt.Errorf("access.yml: top-level 'Admin' key is forbidden, Admin is hardcoded"))
	}
	for groupName, actions := range cfg {
		for actionName, entry := range actions {
			fullName := groupName + "." + actionName
			if entry.Roles == nil {
				continue
			}
			if len(entry.Roles) == 0 {
				panic(fmt.Errorf("access.yml: Action '%s' has an empty roles list, would deny everyone. Use a non-empty list or remove the entry", fullName))
			}
			for _, role := range entry.Roles {
				if role == adminRoleName {
					panic(fmt.Errorf("access.yml: Action '%s' lists 'Admin'. Admin is implicit, remove it from the list", fullName))
				}
				if role == "" {
					panic(fmt.Errorf("access.yml: Action '%s' has an empty role name", fullName))
				}
			}
			for _, tag := range entry.Requires {
				if _, ok := knownTags[tag]; !ok {
					panic(fmt.Errorf("access.yml: Action '%s' has unknown requires tag '%s'. Known tags: [FullAccess]", fullName, tag))
				}
			}
		}
	}
}
