package config

import (
	"cmp"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/hayakawakaki/go-racp/internal/platform/refdata"
)

var themeNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

var purchasePackageKeyRe = regexp.MustCompile(`^[a-z0-9_]+$`)

type RatesConfig struct {
	ExpRate         int `yaml:"ExpRate"`
	JobRate         int `yaml:"JobRate"`
	DropRateCommon  int `yaml:"DropRateCommon"`
	DropRateHeal    int `yaml:"DropRateHeal"`
	DropRateUsable  int `yaml:"DropRateUsable"`
	DropRateEquip   int `yaml:"DropRateEquip"`
	DropRateCard    int `yaml:"DropRateCard"`
	DropRateCardMVP int `yaml:"DropRateCardMVP"`
}

// GeneralConfig holds UI/branding settings shared across every page.
type GeneralConfig struct {
	location   *time.Location
	ServerName string      `yaml:"ServerName"`
	Timezone   string      `yaml:"Timezone"`
	Theme      string      `yaml:"Theme"`
	Rates      RatesConfig `yaml:"Rates"`
	Gepard     bool        `yaml:"Gepard"`
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
	RequireTLS  bool   `yaml:"RequireTLS"`
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
	VerificationResend     time.Duration `yaml:"VerificationResend"`
	PasswordResetRequest   time.Duration `yaml:"PasswordResetRequest"`
	PasswordChange         time.Duration `yaml:"PasswordChange"`
	EmailChangeRequest     time.Duration `yaml:"EmailChangeRequest"`
	EmailChange            time.Duration `yaml:"EmailChange"`
	TicketOpen             time.Duration `yaml:"TicketOpen"`
	CharacterLookReset     time.Duration `yaml:"CharacterLookReset"`
	CharacterLocationReset time.Duration `yaml:"CharacterLocationReset"`
}

// CurrencyConfig tunes the deposit and withdraw bridge.
// Cooldown is a single shared window. Any deposit or withdraw locks both directions for that long.
// MaxZenyPerTx and MaxCashpointPerTx cap a single transaction.
// Cooldown is clamped to [1m, 72h] and the poll intervals to [30s, 15m].
type CurrencyConfig struct {
	Cooldown              time.Duration `yaml:"Cooldown"`
	DepositPollInterval   time.Duration `yaml:"DepositPollInterval"`
	WithdrawDrainInterval time.Duration `yaml:"WithdrawDrainInterval"`
	ReapAfter             time.Duration `yaml:"ReapAfter"`
	MaxZenyPerTx          int64         `yaml:"MaxZenyPerTx"`
	MaxCashpointPerTx     int           `yaml:"MaxCashpointPerTx"`
}

type NotificationConfig struct {
	PruneInterval time.Duration `yaml:"PruneInterval"`
	Retention     time.Duration `yaml:"Retention"`
	RecentLimit   int           `yaml:"RecentLimit"`
}

type PurchasesConfig struct {
	Currency  string          `yaml:"Currency"`
	Packages  []PackageConfig `yaml:"Packages"`
	Providers ProviderFlags   `yaml:"Providers"`
}

type ProviderFlags struct {
	Stripe bool `yaml:"Stripe"`
	Paypal bool `yaml:"Paypal"`
	Crypto bool `yaml:"Crypto"`
}

type PackageConfig struct {
	Key        string `yaml:"Key"`
	Name       string `yaml:"Name"`
	Price      int64  `yaml:"Price"`
	CashPoints int    `yaml:"CashPoints"`
}

type RetentionConfig struct {
	LoginAttempts time.Duration `yaml:"LoginAttempts"`
	SweepInterval time.Duration `yaml:"SweepInterval"`
}

type DefaultLocationConfig struct {
	Map string `yaml:"Map"`
	X   int    `yaml:"X"`
	Y   int    `yaml:"Y"`
}

type TicketLimitsConfig struct {
	MaxOpenPerPlayer int `yaml:"MaxOpenPerPlayer"`
}

type TicketCategoryConfig struct {
	Display string   `yaml:"Display"`
	Roles   []string `yaml:"Roles"`
}

type TicketCategoriesConfig map[string]TicketCategoryConfig

type NewsCategoryConfig struct {
	Display string `yaml:"Display"`
}

type NewsCategoriesConfig map[string]NewsCategoryConfig

type TicketsConfig struct {
	StaffPollInterval time.Duration `yaml:"StaffPollInterval"`
}

type RolesConfig map[string]int

type AuthConfig struct {
	AllowTempBannedLogin bool `yaml:"AllowTempBannedLogin"`
}

type ItemDBConfig struct {
	YAML refdata.SourceGroup `yaml:"YAML"`
	Lua  refdata.SourceGroup `yaml:"Lua"`
}

type MobDBConfig struct {
	YAML refdata.SourceGroup `yaml:"YAML"`
}

type VendorConfig struct {
	PollInterval time.Duration `yaml:"PollInterval"`
}

type MetricsConfig struct {
	PeakWindows         []string      `yaml:"PeakWindows"`
	OnlinePollInterval  time.Duration `yaml:"OnlinePollInterval"`
	GeneralPollInterval time.Duration `yaml:"GeneralPollInterval"`
	StatusPollInterval  time.Duration `yaml:"StatusPollInterval"`
}

type APIKeysConfig struct {
	Tiers map[string]RateLimitRule `yaml:"Tiers"`
}

type SecurityConfig struct {
	TrustedProxyCIDRs     []string `yaml:"TrustedProxyCIDRs"`
	CSPExtraScriptSrc     []string `yaml:"CSPExtraScriptSrc"`
	CSPExtraStyleSrc      []string `yaml:"CSPExtraStyleSrc"`
	CSPExtraImgSrc        []string `yaml:"CSPExtraImgSrc"`
	CSPExtraFormAction    []string `yaml:"CSPExtraFormAction"`
	TrustedOrigins        []string `yaml:"TrustedOrigins"`
	HSTSMaxAge            int      `yaml:"HSTSMaxAge"`
	HSTSIncludeSubdomains bool     `yaml:"HSTSIncludeSubdomains"`
	HSTSPreload           bool     `yaml:"HSTSPreload"`
}

const peakWindowAllTime = "all_time"

var defaultPeakWindows = []string{"daily", "weekly", "monthly", peakWindowAllTime}

// AppConfig holds operator-tunable application settings loaded from config.yml.
//
//nolint:govet // fieldalignment: 8-byte gain inside a singleton config
type AppConfig struct {
	UserRoles        RolesConfig            `yaml:"UserRoles"`
	TicketCategories TicketCategoriesConfig `yaml:"TicketCategories"`
	NewsCategories   NewsCategoriesConfig   `yaml:"NewsCategories"`
	General          GeneralConfig          `yaml:"App"`
	Mailer           MailerConfig           `yaml:"Mailer"`
	DefaultLocation  DefaultLocationConfig  `yaml:"DefaultLocation"`
	ItemDB           ItemDBConfig           `yaml:"ItemDB"`
	MobDB            MobDBConfig            `yaml:"MobDB"`
	Cooldown         CooldownConfig         `yaml:"Cooldown"`
	Currency         CurrencyConfig         `yaml:"Currency"`
	Notifications    NotificationConfig     `yaml:"Notifications"`
	Purchases        PurchasesConfig        `yaml:"Purchases"`
	Retention        RetentionConfig        `yaml:"Retention"`
	TTL              TTLConfig              `yaml:"TTL"`
	Tickets          TicketsConfig          `yaml:"Tickets"`
	TicketLimits     TicketLimitsConfig     `yaml:"TicketLimits"`
	Auth             AuthConfig             `yaml:"Auth"`
	Vendor           VendorConfig           `yaml:"Vendor"`
	Metrics          MetricsConfig          `yaml:"Metrics"`
	Security         SecurityConfig         `yaml:"Security"`
	APIKeys          APIKeysConfig          `yaml:"APIKeys"`
	clampWarnings    []ClampAdjustment
}

func (c AppConfig) ClampWarnings() []ClampAdjustment {
	return slices.Clone(c.clampWarnings)
}

// appConfigDefaults apply default config in case of missing config file
func appConfigDefaults() *AppConfig {
	return &AppConfig{
		General: GeneralConfig{
			ServerName: "Go Control Panel",
			Timezone:   "UTC",
			Theme:      "default",
			Rates: RatesConfig{
				ExpRate:         100,
				JobRate:         100,
				DropRateCommon:  100,
				DropRateHeal:    100,
				DropRateUsable:  100,
				DropRateEquip:   100,
				DropRateCard:    100,
				DropRateCardMVP: 100,
			},
		},
		Mailer: MailerConfig{FromAddress: "noreply@gocp.com"},
		TTL: TTLConfig{
			Session:       24 * time.Hour,
			Verification:  30 * time.Minute,
			PasswordReset: 1 * time.Hour,
			EmailChange:   24 * time.Hour,
		},
		Cooldown: CooldownConfig{
			VerificationResend:     60 * time.Second,
			PasswordResetRequest:   30 * time.Minute,
			PasswordChange:         7 * 24 * time.Hour,
			EmailChangeRequest:     60 * time.Second,
			EmailChange:            14 * 24 * time.Hour,
			TicketOpen:             5 * time.Minute,
			CharacterLookReset:     24 * time.Hour,
			CharacterLocationReset: 1 * time.Hour,
		},
		Currency: CurrencyConfig{
			Cooldown:              5 * time.Minute,
			MaxZenyPerTx:          2_000_000_000,
			MaxCashpointPerTx:     1_000_000,
			DepositPollInterval:   30 * time.Second,
			WithdrawDrainInterval: 30 * time.Second,
			ReapAfter:             30 * time.Minute,
		},
		Notifications: NotificationConfig{
			PruneInterval: time.Hour,
			Retention:     30 * 24 * time.Hour,
			RecentLimit:   20,
		},
		Retention: RetentionConfig{
			LoginAttempts: 30 * 24 * time.Hour,
			SweepInterval: 1 * time.Hour,
		},
		UserRoles:    RolesConfig{"Moderator": 20, "Enforcer": 10, "Event": 2},
		TicketLimits: TicketLimitsConfig{MaxOpenPerPlayer: 5},
		TicketCategories: TicketCategoriesConfig{
			"Other": {Display: "Other", Roles: []string{"*"}},
		},
		NewsCategories: NewsCategoriesConfig{
			"Announcement": {Display: "Announcement"},
		},
		Tickets:         TicketsConfig{StaffPollInterval: 30 * time.Second},
		Auth:            AuthConfig{AllowTempBannedLogin: true},
		DefaultLocation: DefaultLocationConfig{Map: "prontera", X: 156, Y: 191},
		ItemDB:          ItemDBConfig{},
		MobDB:           MobDBConfig{},
		Vendor:          VendorConfig{PollInterval: 30 * time.Second},
		Metrics: MetricsConfig{
			OnlinePollInterval:  1 * time.Minute,
			GeneralPollInterval: 1 * time.Hour,
			StatusPollInterval:  1 * time.Minute,
			PeakWindows:         defaultPeakWindows,
		},
		Security: SecurityConfig{
			TrustedProxyCIDRs:     []string{"127.0.0.1/32", "::1/128", "172.16.0.0/12"},
			CSPExtraImgSrc:        []string{"https://i.imgur.com"},
			HSTSMaxAge:            31536000,
			HSTSIncludeSubdomains: true,
		},
		APIKeys: APIKeysConfig{
			Tiers: map[string]RateLimitRule{
				"Standard": {RatePerMinute: 100, Burst: 100},
				"Elevated": {RatePerMinute: 1000, Burst: 1000},
			},
		},
	}
}

var appConfigFiles = []string{
	"app.yml",
	"auth.yml",
	"security.yml",
	"roles.yml",
	"tickets.yml",
	"news.yml",
	"datasources.yml",
	"polling.yml",
	"purchases.yml",
	"notifications.yml",
	"apikeys.yml",
}

func ProcessAppConfig() *AppConfig {
	anchor, err := GetTargetFilePath("conf/" + appConfigFiles[0])
	if err != nil {
		panic(fmt.Errorf("locate conf/%s: %w", appConfigFiles[0], err))
	}

	return loadAppConfigFromDir(filepath.Dir(anchor))
}

func loadAppConfigFromDir(dir string) *AppConfig {
	cfg := appConfigDefaults()
	for _, name := range appConfigFiles {
		path := filepath.Join(dir, name)
		//nolint:gosec // G304: path is built from a fixed allowlist (appConfigFiles) joined to an operator-controlled directory.
		data, err := os.ReadFile(path)
		if err != nil {
			panic(fmt.Errorf("read conf/%s: %w", name, err))
		}
		if len(data) == 0 {
			continue
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			panic(fmt.Errorf("parse conf/%s: %w", name, err))
		}
	}
	validateAppConfig(cfg)
	return cfg
}

func validateAppConfig(cfg *AppConfig) {
	durations := map[string]time.Duration{
		"TTL.Session":                     cfg.TTL.Session,
		"TTL.Verification":                cfg.TTL.Verification,
		"TTL.PasswordReset":               cfg.TTL.PasswordReset,
		"TTL.EmailChange":                 cfg.TTL.EmailChange,
		"Cooldown.VerificationResend":     cfg.Cooldown.VerificationResend,
		"Cooldown.PasswordResetRequest":   cfg.Cooldown.PasswordResetRequest,
		"Cooldown.PasswordChange":         cfg.Cooldown.PasswordChange,
		"Cooldown.EmailChangeRequest":     cfg.Cooldown.EmailChangeRequest,
		"Cooldown.EmailChange":            cfg.Cooldown.EmailChange,
		"Cooldown.TicketOpen":             cfg.Cooldown.TicketOpen,
		"Cooldown.CharacterLookReset":     cfg.Cooldown.CharacterLookReset,
		"Cooldown.CharacterLocationReset": cfg.Cooldown.CharacterLocationReset,
		"Retention.LoginAttempts":         cfg.Retention.LoginAttempts,
		"Retention.SweepInterval":         cfg.Retention.SweepInterval,
	}
	for name, value := range durations {
		if value <= 0 {
			panic(fmt.Errorf("%s must be > 0, got %v", name, value))
		}
	}
	if cfg.Mailer.FromAddress == "" {
		panic(fmt.Errorf("Mailer.FromAddress is required"))
	}
	if cfg.General.Timezone == "" {
		panic(fmt.Errorf("App.Timezone is required"))
	}
	loc, err := time.LoadLocation(cfg.General.Timezone)
	if err != nil {
		panic(fmt.Errorf("App.Timezone %q is not a valid IANA timezone: %w", cfg.General.Timezone, err))
	}
	cfg.General.location = loc

	if cfg.DefaultLocation.Map == "" {
		panic(fmt.Errorf("DefaultLocation.Map is required"))
	}
	if cfg.DefaultLocation.X <= 0 || cfg.DefaultLocation.Y <= 0 {
		panic(fmt.Errorf("DefaultLocation.X and DefaultLocation.Y must be > 0"))
	}

	var clamps []ClampAdjustment
	validateRolesConfig(cfg.UserRoles)
	validateTicketsConfig(cfg.TicketCategories, cfg.TicketLimits, cfg.Tickets, cfg.UserRoles)
	validateNewsConfig(cfg.NewsCategories)
	validateVendorConfig(&cfg.Vendor, &clamps)
	validateMetricsConfig(&cfg.Metrics, &clamps)
	validateNotificationsConfig(&cfg.Notifications, &clamps)
	validateTrustedProxyCIDRs(cfg.Security.TrustedProxyCIDRs)
	validateTheme(&cfg.General)
	validateRatesConfig(&cfg.General.Rates)
	validateCurrencyConfig(&cfg.Currency, &clamps)
	validatePurchasesConfig(&cfg.Purchases)
	validateAPIKeysConfig(&cfg.APIKeys)
	cfg.clampWarnings = clamps
}

func validateAPIKeysConfig(cfg *APIKeysConfig) {
	for tier, rule := range cfg.Tiers {
		if tier == "" {
			panic(fmt.Errorf("APIKeys.Tiers has an empty tier name"))
		}
		if rule.RatePerMinute <= 0 {
			panic(fmt.Errorf("APIKeys.Tiers.%s.RatePerMinute %d, must be > 0", tier, rule.RatePerMinute))
		}
		if rule.Burst <= 0 {
			panic(fmt.Errorf("APIKeys.Tiers.%s.Burst %d, must be > 0", tier, rule.Burst))
		}
	}
}

func validatePurchasesConfig(cfg *PurchasesConfig) {
	if len(cfg.Packages) == 0 {
		return
	}
	if len(cfg.Currency) != 3 {
		panic(fmt.Errorf("Purchases.Currency must be a 3-letter ISO code, got %q", cfg.Currency))
	}

	seen := make(map[string]struct{}, len(cfg.Packages))
	for _, pkg := range cfg.Packages {
		if !purchasePackageKeyRe.MatchString(pkg.Key) {
			panic(fmt.Errorf("Purchases.Packages key %q must match %s", pkg.Key, purchasePackageKeyRe))
		}
		if _, dup := seen[pkg.Key]; dup {
			panic(fmt.Errorf("Purchases.Packages key %q is duplicated", pkg.Key))
		}
		seen[pkg.Key] = struct{}{}
		if pkg.Name == "" {
			panic(fmt.Errorf("Purchases.Packages %q must have a name", pkg.Key))
		}
		if pkg.Price <= 0 {
			panic(fmt.Errorf("Purchases.Packages %q Price must be > 0", pkg.Key))
		}
		if pkg.CashPoints <= 0 {
			panic(fmt.Errorf("Purchases.Packages %q CashPoints must be > 0", pkg.Key))
		}
	}
}

func validateCurrencyConfig(cfg *CurrencyConfig, adjustments *[]ClampAdjustment) {
	const (
		minCooldown = 1 * time.Minute
		maxCooldown = 72 * time.Hour
		minPoll     = 30 * time.Second
		maxPoll     = 15 * time.Minute
	)

	cfg.Cooldown = recordClamp(adjustments, "Currency.Cooldown", cfg.Cooldown, 5*time.Minute, minCooldown, maxCooldown)
	cfg.DepositPollInterval = recordClamp(adjustments, "Currency.DepositPollInterval", cfg.DepositPollInterval, 30*time.Second, minPoll, maxPoll)
	cfg.WithdrawDrainInterval = recordClamp(adjustments, "Currency.WithdrawDrainInterval", cfg.WithdrawDrainInterval, 30*time.Second, minPoll, maxPoll)
	cfg.ReapAfter = recordClamp(adjustments, "Currency.ReapAfter", cfg.ReapAfter, 30*time.Minute, 5*time.Minute, 24*time.Hour)

	if cfg.MaxZenyPerTx <= 0 {
		panic(fmt.Errorf("Currency.MaxZenyPerTx must be > 0"))
	}
	if cfg.MaxCashpointPerTx <= 0 {
		panic(fmt.Errorf("Currency.MaxCashpointPerTx must be > 0"))
	}
}

func validateTheme(cfg *GeneralConfig) {
	cfg.Theme = cmp.Or(cfg.Theme, "default")
	if !themeNameRe.MatchString(cfg.Theme) {
		panic(fmt.Errorf("App.Theme %q must match %s", cfg.Theme, themeNameRe))
	}
}

func validateTrustedProxyCIDRs(cidrs []string) {
	for _, cidr := range cidrs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			panic(fmt.Errorf("Security.TrustedProxyCIDRs entry %q is not a valid CIDR: %w", cidr, err))
		}
	}
}

type ClampAdjustment struct {
	Field   string
	Given   time.Duration
	Clamped time.Duration
}

func recordClamp(adjustments *[]ClampAdjustment, field string, value, fallback, minimum, maximum time.Duration) time.Duration {
	clamped := clampInterval(value, fallback, minimum, maximum)
	if value > 0 && clamped != value {
		*adjustments = append(*adjustments, ClampAdjustment{Field: field, Given: value, Clamped: clamped})
	}
	return clamped
}

func clampInterval(value, fallback, minimum, maximum time.Duration) time.Duration {
	switch {
	case value <= 0:
		return fallback
	case value < minimum:
		return minimum
	case value > maximum:
		return maximum
	default:
		return value
	}
}

func validateMetricsConfig(cfg *MetricsConfig, adjustments *[]ClampAdjustment) {
	const (
		minOnline  = 10 * time.Second
		maxOnline  = 1 * time.Hour
		minGeneral = 5 * time.Minute
		maxGeneral = 24 * time.Hour
		minStatus  = 10 * time.Second
		maxStatus  = 1 * time.Hour
	)

	cfg.OnlinePollInterval = recordClamp(adjustments, "Metrics.OnlinePollInterval", cfg.OnlinePollInterval, 1*time.Minute, minOnline, maxOnline)
	cfg.GeneralPollInterval = recordClamp(adjustments, "Metrics.GeneralPollInterval", cfg.GeneralPollInterval, 1*time.Hour, minGeneral, maxGeneral)
	cfg.StatusPollInterval = recordClamp(adjustments, "Metrics.StatusPollInterval", cfg.StatusPollInterval, 1*time.Minute, minStatus, maxStatus)

	if cfg.PeakWindows == nil {
		cfg.PeakWindows = defaultPeakWindows
		return
	}
	allowed := map[string]struct{}{
		"daily": {}, "weekly": {}, "monthly": {}, peakWindowAllTime: {},
	}
	filtered := make([]string, 0, len(cfg.PeakWindows))
	for _, w := range cfg.PeakWindows {
		if _, ok := allowed[w]; ok {
			filtered = append(filtered, w)
		}
	}
	cfg.PeakWindows = filtered
}

func validateRatesConfig(cfg *RatesConfig) {
	for _, rate := range []*int{
		&cfg.ExpRate,
		&cfg.JobRate,
		&cfg.DropRateCommon,
		&cfg.DropRateHeal,
		&cfg.DropRateUsable,
		&cfg.DropRateEquip,
		&cfg.DropRateCard,
		&cfg.DropRateCardMVP,
	} {
		if *rate <= 0 {
			*rate = 100
		}
	}
}

func validateVendorConfig(cfg *VendorConfig, adjustments *[]ClampAdjustment) {
	const (
		minInterval = 5 * time.Second
		maxInterval = 10 * time.Minute
	)
	cfg.PollInterval = recordClamp(adjustments, "Vendor.PollInterval", cfg.PollInterval, 30*time.Second, minInterval, maxInterval)
}

func validateNotificationsConfig(cfg *NotificationConfig, adjustments *[]ClampAdjustment) {
	const (
		minPrune     = 1 * time.Minute
		maxPrune     = 24 * time.Hour
		minRetention = 1 * time.Hour
		maxRetention = 365 * 24 * time.Hour
		minRecent    = 1
		maxRecent    = 100
	)
	cfg.PruneInterval = recordClamp(adjustments, "Notifications.PruneInterval", cfg.PruneInterval, time.Hour, minPrune, maxPrune)
	cfg.Retention = recordClamp(adjustments, "Notifications.Retention", cfg.Retention, 30*24*time.Hour, minRetention, maxRetention)

	if cfg.RecentLimit < minRecent {
		cfg.RecentLimit = 20
	} else if cfg.RecentLimit > maxRecent {
		cfg.RecentLimit = maxRecent
	}
}

func validateNewsConfig(categories NewsCategoriesConfig) {
	if len(categories) == 0 {
		panic(fmt.Errorf("NewsCategories must define at least one category"))
	}
	for key, category := range categories {
		if strings.TrimSpace(category.Display) == "" {
			panic(fmt.Errorf("NewsCategories.%s.Display is required", key))
		}
	}
}

func validateTicketsConfig(
	categories TicketCategoriesConfig,
	limits TicketLimitsConfig,
	tickets TicketsConfig,
	roles RolesConfig,
) {
	if len(categories) == 0 {
		panic(fmt.Errorf("TicketCategories must define at least one category"))
	}
	if limits.MaxOpenPerPlayer < 1 {
		panic(fmt.Errorf("TicketLimits.MaxOpenPerPlayer must be >= 1, got %d", limits.MaxOpenPerPlayer))
	}
	if tickets.StaffPollInterval <= 0 {
		panic(fmt.Errorf("Tickets.StaffPollInterval must be > 0, got %v", tickets.StaffPollInterval))
	}
	for key, category := range categories {
		validateTicketCategory(key, category, roles)
	}
}

func validateTicketCategory(key string, category TicketCategoryConfig, roles RolesConfig) {
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

var reservedRoleNames = map[string]struct{}{
	"Admin":  {},
	"Player": {},
	"Public": {},
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
