package config

import (
	"fmt"
	"os"
	"time"

	"github.com/goccy/go-yaml"
)

// GeneralConfig holds UI/branding settings shared across every page.
type GeneralConfig struct {
	location   *time.Location
	ServerName string `yaml:"ServerName"`
	Timezone   string `yaml:"Timezone"`
}

func (g GeneralConfig) Location() *time.Location {
	if g.location == nil {
		return time.UTC
	}
	return g.location
}

// MailerConfig holds outgoing-mail settings.
type MailerConfig struct {
	FromAddress string `yaml:"FromAddress"`
}

// TTLConfig groups every lifetime knob (HTTP session + action-token lifetimes).
type TTLConfig struct {
	Session       time.Duration `yaml:"Session"`
	Verification  time.Duration `yaml:"Verification"`
	PasswordReset time.Duration `yaml:"PasswordReset"`
	EmailChange   time.Duration `yaml:"EmailChange"`
}

// CooldownConfig groups per-flow rate-limit windows.
// VerificationResend, PasswordResetRequest, EmailChangeRequest are request rate limits
// (caps on how often a new token can be issued for a given account+action).
// PasswordChange and EmailChange are post-success lockouts (caps on how often the
// underlying credential can actually be mutated).
type CooldownConfig struct {
	VerificationResend   time.Duration `yaml:"VerificationResend"`
	PasswordResetRequest time.Duration `yaml:"PasswordResetRequest"`
	PasswordChange       time.Duration `yaml:"PasswordChange"`
	EmailChangeRequest   time.Duration `yaml:"EmailChangeRequest"`
	EmailChange          time.Duration `yaml:"EmailChange"`
	TicketOpen           time.Duration `yaml:"TicketOpen"`
}

type TicketLimitsConfig struct {
	MaxOpenPerPlayer int `yaml:"MaxOpenPerPlayer"`
}

type TicketCategoryConfig struct {
	Display string   `yaml:"Display"`
	Roles   []string `yaml:"Roles"`
}

type TicketCategoriesConfig map[string]TicketCategoryConfig

type TicketsConfig struct {
	StaffPollInterval time.Duration `yaml:"StaffPollInterval"`
}

type RolesConfig map[string]int

// AppConfig holds operator-tunable application settings loaded from config.yml.
type AppConfig struct {
	General          GeneralConfig          `yaml:"GeneralConfig"`
	UserRoles        RolesConfig            `yaml:"UserRoles"`
	TicketCategories TicketCategoriesConfig `yaml:"TicketCategories"`
	Mailer           MailerConfig           `yaml:"MailerConfig"`
	Cooldown         CooldownConfig         `yaml:"Cooldown"`
	TTL              TTLConfig              `yaml:"TTL"`
	Tickets          TicketsConfig          `yaml:"Tickets"`
	TicketLimits     TicketLimitsConfig     `yaml:"TicketLimits"`
}

// appConfigDefaults apply default config in case of missing config file
func appConfigDefaults() *AppConfig {
	return &AppConfig{
		General: GeneralConfig{ServerName: "Go Control Panel", Timezone: "UTC"},
		Mailer:  MailerConfig{FromAddress: "noreply@gocp.com"},
		TTL: TTLConfig{
			Session:       24 * time.Hour,
			Verification:  30 * time.Minute,
			PasswordReset: 1 * time.Hour,
			EmailChange:   24 * time.Hour,
		},
		Cooldown: CooldownConfig{
			VerificationResend:   60 * time.Second,
			PasswordResetRequest: 30 * time.Minute,
			PasswordChange:       7 * 24 * time.Hour,
			EmailChangeRequest:   60 * time.Second,
			EmailChange:          14 * 24 * time.Hour,
			TicketOpen:           5 * time.Minute,
		},
		UserRoles:    RolesConfig{"Moderator": 20, "Enforcer": 10, "Event": 2},
		TicketLimits: TicketLimitsConfig{MaxOpenPerPlayer: 5},
		TicketCategories: TicketCategoriesConfig{
			"Other": {Display: "Other", Roles: []string{"*"}},
		},
		Tickets: TicketsConfig{StaffPollInterval: 30 * time.Second},
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

	validateAppConfig(cfg)
	return cfg
}

func validateAppConfig(cfg *AppConfig) {
	durations := map[string]time.Duration{
		"TTL.Session":                   cfg.TTL.Session,
		"TTL.Verification":              cfg.TTL.Verification,
		"TTL.PasswordReset":             cfg.TTL.PasswordReset,
		"TTL.EmailChange":               cfg.TTL.EmailChange,
		"Cooldown.VerificationResend":   cfg.Cooldown.VerificationResend,
		"Cooldown.PasswordResetRequest": cfg.Cooldown.PasswordResetRequest,
		"Cooldown.PasswordChange":       cfg.Cooldown.PasswordChange,
		"Cooldown.EmailChangeRequest":   cfg.Cooldown.EmailChangeRequest,
		"Cooldown.EmailChange":          cfg.Cooldown.EmailChange,
		"Cooldown.TicketOpen":           cfg.Cooldown.TicketOpen,
	}
	for name, value := range durations {
		if value <= 0 {
			panic(fmt.Errorf("%s must be > 0, got %v", name, value))
		}
	}
	if cfg.Mailer.FromAddress == "" {
		panic(fmt.Errorf("MailerConfig.FromAddress is required"))
	}
	if cfg.General.Timezone == "" {
		panic(fmt.Errorf("GeneralConfig.Timezone is required"))
	}
	loc, err := time.LoadLocation(cfg.General.Timezone)
	if err != nil {
		panic(fmt.Errorf("GeneralConfig.Timezone %q is not a valid IANA timezone: %w", cfg.General.Timezone, err))
	}
	cfg.General.location = loc

	validateRolesConfig(cfg.UserRoles)
	validateTicketsConfig(cfg.TicketCategories, cfg.TicketLimits, cfg.Tickets, cfg.UserRoles)
}

func validateTicketsConfig(
	categories TicketCategoriesConfig,
	limits TicketLimitsConfig,
	tickets TicketsConfig,
	roles RolesConfig,
) {
	if limits.MaxOpenPerPlayer < 1 {
		panic(fmt.Errorf("TicketLimits.MaxOpenPerPlayer must be >= 1, got %d", limits.MaxOpenPerPlayer))
	}
	if tickets.StaffPollInterval <= 0 {
		panic(fmt.Errorf("Tickets.StaffPollInterval must be > 0, got %v", tickets.StaffPollInterval))
	}
	for key, category := range categories {
		if category.Display == "" {
			panic(fmt.Errorf("TicketCategories.%s.Display is required", key))
		}
		if len(category.Roles) == 0 {
			panic(fmt.Errorf("TicketCategories.%s.Roles must list at least one role (or [\"*\"])", key))
		}
		for _, role := range category.Roles {
			if role == "*" || role == adminRoleName {
				continue
			}
			if _, ok := roles[role]; !ok {
				panic(fmt.Errorf("TicketCategories.%s.Roles references unknown role %q (declare it in UserRoles)", key, role))
			}
		}
	}
}

var reservedRoleNames = map[string]struct{}{
	"Admin":  {},
	"Player": {},
	"*":      {},
}

func validateRolesConfig(roles RolesConfig) {
	seenGroupIDs := map[int]string{}
	for name, value := range roles {
		if name == "" {
			panic(fmt.Errorf("user roles config: empty role name"))
		}
		qualified := "UserRoles." + name
		if _, reserved := reservedRoleNames[name]; reserved {
			panic(fmt.Errorf("%s is reserved and cannot be redefined", qualified))
		}
		if value <= 0 {
			panic(fmt.Errorf("%s must be > 0, got %d", qualified, value))
		}
		if value == 99 {
			panic(fmt.Errorf("%s = 99 is reserved for admin", qualified))
		}
		if previousName, dup := seenGroupIDs[value]; dup {
			panic(fmt.Errorf("%s shares group_id %d with UserRoles.%s", qualified, value, previousName))
		}
		seenGroupIDs[value] = name
	}
}
