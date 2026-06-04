package config

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"

	"github.com/goccy/go-yaml"
)

type RoleList []string

const RequireUnrestricted = "Unrestricted"

const requireKeyTag = "APIKey"

const publicRoleName = "Public"

const MemberRoleName = "Member"

const VerifiedRoleName = "Verified"

type Entry struct {
	Roles     RoleList
	RateLimit *RateLimitRule
	Requires  []string
}

type RateLimitRule struct {
	RatePerMinute int `yaml:"RatePerMinute"`
	Burst         int `yaml:"Burst"`
}

type ActionRoles map[string]Entry

type AccessConfig map[string]ActionRoles

func (e Entry) RequiresUnrestricted() bool {
	return slices.Contains(e.Requires, RequireUnrestricted)
}

func (e Entry) RequiresAPIKey() bool {
	return slices.Contains(e.Requires, requireKeyTag)
}

func (e Entry) RequiresVerified() bool {
	for _, role := range e.Roles {
		if role == publicRoleName || role == MemberRoleName {
			return false
		}
	}

	return true
}

var entryAllowedKeys = map[string]struct{}{
	"Roles":     {},
	"Requires":  {},
	"RateLimit": {},
}

func (e *Entry) UnmarshalYAML(unmarshal func(any) error) error {
	var asList RoleList
	if err := unmarshal(&asList); err == nil {
		e.Roles = asList
		e.Requires = nil
		return nil
	}

	var raw map[string]any
	if err := unmarshal(&raw); err != nil {
		return fmt.Errorf("access.yml entry: expected list of roles or { Roles, Requires, RateLimit }: %w", err)
	}
	for key := range raw {
		if _, ok := entryAllowedKeys[key]; !ok {
			return fmt.Errorf("access.yml entry: unknown field %q (allowed: Roles, Requires, RateLimit)", key)
		}
	}

	var asStruct struct {
		Roles     RoleList       `yaml:"Roles"`
		RateLimit *RateLimitRule `yaml:"RateLimit"`
		Requires  []string       `yaml:"Requires"`
	}
	if err := unmarshal(&asStruct); err != nil {
		return fmt.Errorf("access.yml entry: expected list of roles or { Roles, Requires, RateLimit }: %w", err)
	}
	e.Roles = asStruct.Roles
	e.Requires = asStruct.Requires
	e.RateLimit = asStruct.RateLimit

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
func ProcessAccessConfig(themeData []byte) AccessConfig {
	rootCfg := AccessConfig{}

	cfgPath, err := GetTargetFilePath("conf/access.yml")
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			panic(fmt.Errorf("locate access.yml: %w", err))
		}
	} else {
		rootCfg, err = loadAccessConfig(cfgPath)
		if err != nil {
			panic(err)
		}
	}

	themeCfg, err := parseAccessConfig(themeData)
	if err != nil {
		panic(fmt.Errorf("parse theme access.yml: %w", err))
	}

	validateThemeAccessConfig(themeCfg)

	merged := mergeAccessConfigs(rootCfg, themeCfg)

	validateAccessConfig(merged)

	return merged
}

func validateThemeAccessConfig(cfg AccessConfig) {
	for group := range cfg {
		if group != "ThemePages" {
			panic(fmt.Errorf("theme access.yml: group %q is not permitted, only ThemePages.* entries are allowed (move slice tags to root access.yml)", group))
		}
	}
}

func mergeAccessConfigs(root, theme AccessConfig) AccessConfig {
	out := AccessConfig{}

	for group, actions := range root {
		out[group] = maps.Clone(actions)
	}

	for group, actions := range theme {
		existing, hasGroup := out[group]
		if !hasGroup {
			existing = ActionRoles{}
		}

		for action, entry := range actions {
			if _, dup := existing[action]; dup {
				panic(fmt.Errorf("access.yml: %s.%s defined in both root access.yml and theme access.yml, remove one", group, action))
			}

			existing[action] = entry
		}

		out[group] = existing
	}

	return out
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

//nolint:cyclop // splitting would obscure the flow
func validateAccessConfig(cfg AccessConfig) {
	knownTags := map[string]struct{}{RequireUnrestricted: {}, requireKeyTag: {}}

	if _, hasAdmin := cfg[adminRoleName]; hasAdmin {
		panic(fmt.Errorf("access.yml: top-level 'Admin' key is forbidden, Admin is hardcoded"))
	}
	for groupName, actions := range cfg {
		for actionName, entry := range actions {
			fullName := groupName + "." + actionName
			for _, tag := range entry.Requires {
				if _, ok := knownTags[tag]; !ok {
					panic(fmt.Errorf("access.yml: Action '%s' has unknown requires tag '%s'. Known tags: [%s, %s]", fullName, tag, RequireUnrestricted, requireKeyTag))
				}
			}

			if entry.RateLimit != nil {
				if entry.RateLimit.RatePerMinute <= 0 {
					panic(fmt.Errorf("access.yml: Action '%s' has RateLimit.RatePerMinute %d, must be > 0", fullName, entry.RateLimit.RatePerMinute))
				}
				if entry.RateLimit.Burst <= 0 {
					panic(fmt.Errorf("access.yml: Action '%s' has RateLimit.Burst %d, must be > 0", fullName, entry.RateLimit.Burst))
				}
			}

			if entry.Roles == nil {
				continue
			}
			if len(entry.Roles) == 0 {
				panic(fmt.Errorf("access.yml: Action '%s' has an empty roles list, would deny everyone. Use a non-empty list or remove the entry", fullName))
			}
			hasPublic := slices.Contains(entry.Roles, publicRoleName)
			if entry.RequiresAPIKey() && !hasPublic {
				panic(fmt.Errorf("access.yml: Action '%s' has Requires: ['APIKey'] but is not Public. The API key gate only applies to Public routes", fullName))
			}
			if hasPublic && len(entry.Roles) > 1 {
				panic(fmt.Errorf("access.yml: Action '%s' lists 'Public' alongside other roles. Public bypasses auth and cannot be combined", fullName))
			}
			if hasPublic && entry.RequiresUnrestricted() {
				panic(fmt.Errorf("access.yml: Action '%s' combines 'Public' with Requires: ['Unrestricted']. Public bypasses auth, there is no user to classify", fullName))
			}
			soleAdmin := len(entry.Roles) == 1 && entry.Roles[0] == adminRoleName
			for _, role := range entry.Roles {
				if role == adminRoleName && !soleAdmin {
					panic(fmt.Errorf("access.yml: Action '%s' lists 'Admin' alongside other roles. Admin is implicit, remove it from the list", fullName))
				}
				if role == "" {
					panic(fmt.Errorf("access.yml: Action '%s' has an empty role name", fullName))
				}
			}
			if soleAdmin && entry.RequiresUnrestricted() {
				panic(fmt.Errorf("access.yml: Action '%s' combines sole 'Admin' role with Requires: ['Unrestricted']. Admin policy is always Unrestricted; remove the Requires tag", fullName))
			}
		}
	}
}
