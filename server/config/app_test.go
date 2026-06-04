package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "conf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

func validConfFiles() map[string]string {
	return map[string]string{
		"app.yml": `App:
  ServerName: "Test Panel"
  Timezone: "UTC"
  Theme: "default"
  Gepard: false
  Branding:
    Logo: ""
    Discord: "https://discord.gg/test"
  Navbar:
    Items:
      - Label: "Home"
        Href: "/"
        Icon: "home"
Mailer:
  FromAddress: "noreply@test.example"
DefaultLocation:
  Map: "prontera"
  X: 156
  Y: 191
`,
		"auth.yml": `Auth:
  AllowTempBannedLogin: true
TTL:
  Session: "24h"
  Verification: "30m"
  PasswordReset: "1h"
  EmailChange: "24h"
Cooldown:
  VerificationResend: "60s"
  PasswordResetRequest: "30m"
  PasswordChange: "168h"
  EmailChangeRequest: "60s"
  EmailChange: "336h"
  TicketOpen: "30m"
  CharacterLookReset: "24h"
  CharacterLocationReset: "1h"
Retention:
  LoginAttempts: "720h"
  SweepInterval: "1h"
`,
		"security.yml": `Security:
  TrustedProxyCIDRs: ["127.0.0.1/32"]
  HSTSMaxAge: 604800
  HSTSIncludeSubdomains: true
  HSTSPreload: false
  CSPExtraScriptSrc: []
  CSPExtraStyleSrc: []
  CSPExtraImgSrc: []
  TrustedOrigins: []
`,
		"roles.yml": `UserRoles:
  Moderator: 20
  Enforcer: 10
`,
		"tickets.yml": `TicketLimits:
  MaxOpenPerPlayer: 5
TicketCategories:
  Other:
    Display: "Other"
    Roles: ["*"]
Tickets:
  StaffPollInterval: "30s"
`,
		"news.yml": `NewsCategories:
  Announcement:
    Display: "Announcement"
`,
		"datasources.yml": `ItemDB:
  YAML:
    Files: ["item_db.yml"]
MobDB:
  YAML:
    Files: ["mob_db.yml"]
`,
		"polling.yml": `Vendor:
  PollInterval: "30s"
Metrics:
  OnlinePollInterval: "1m"
  GeneralPollInterval: "1h"
  PeakWindows: ["daily", "weekly", "monthly", "all_time"]
`,
		"purchases.yml": `Purchases:
  Currency: "USD"
  Providers:
    Stripe: false
    Paypal: false
    Crypto: false
  Packages:
    - Key: "starter"
      Name: "Starter Pack"
      Price: 5
      CashPoints: 500
`,
		"apikeys.yml": `APIKeys:
  Tiers:
    Standard:
      RatePerMinute: 180
      Burst: 180
    Elevated:
      RatePerMinute: 600
      Burst: 600
`,
		"notifications.yml": `Notifications:
  PruneInterval: 1h
  Retention: 720h
  RecentLimit: 20
`,
	}
}

func mustPanic(t *testing.T, fn func()) string {
	t.Helper()
	var got any
	func() {
		defer func() { got = recover() }()
		fn()
	}()
	if got == nil {
		t.Fatalf("expected panic, got none")
	}
	switch value := got.(type) {
	case error:
		return value.Error()
	case string:
		return value
	default:
		t.Fatalf("unexpected panic type %T: %v", got, got)
		return ""
	}
}

func TestValidateRolesConfig_AcceptsDistinctPositiveNonReservedValues(t *testing.T) {
	t.Parallel()
	cfg := RolesConfig{"Moderator": 20, "Enforcer": 10, "Event": 2, "VIP": 5}
	validateRolesConfig(cfg)
}

func TestValidateRolesConfig_AcceptsEmptyMap(t *testing.T) {
	t.Parallel()
	validateRolesConfig(RolesConfig{})
}

func TestValidateRolesConfig_RejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cfg         RolesConfig
		name        string
		wantContain string
	}{
		{
			name:        "zero value",
			cfg:         RolesConfig{"Moderator": 0},
			wantContain: "UserRoles.Moderator must be > 0",
		},
		{
			name:        "negative value",
			cfg:         RolesConfig{"Enforcer": -1},
			wantContain: "UserRoles.Enforcer must be > 0",
		},
		{
			name:        "reserved 99 for non-admin role",
			cfg:         RolesConfig{"Moderator": 99},
			wantContain: "UserRoles.Moderator = 99 is reserved for admin",
		},
		{
			name:        "reserved name Admin",
			cfg:         RolesConfig{"Admin": 50},
			wantContain: "UserRoles.Admin is reserved",
		},
		{
			name:        "reserved name Player",
			cfg:         RolesConfig{"Player": 50},
			wantContain: "UserRoles.Player is reserved",
		},
		{
			name:        "reserved name Public",
			cfg:         RolesConfig{"Public": 50},
			wantContain: "UserRoles.Public is reserved",
		},
		{
			name:        "reserved name Member",
			cfg:         RolesConfig{"Member": 50},
			wantContain: "UserRoles.Member is reserved",
		},
		{
			name:        "reserved name Verified",
			cfg:         RolesConfig{"Verified": 50},
			wantContain: "UserRoles.Verified is reserved",
		},
		{
			name:        "duplicate group_id",
			cfg:         RolesConfig{"Moderator": 10, "Enforcer": 10},
			wantContain: "shares group_id 10",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := tt.cfg
			msg := mustPanic(t, func() { validateRolesConfig(cfg) })
			if !strings.Contains(msg, tt.wantContain) {
				t.Errorf("panic message = %q, want substring %q", msg, tt.wantContain)
			}
		})
	}
}

func TestValidateVendorConfig_Clamps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{name: "zero defaults to 30s", in: 0, want: 30 * time.Second},
		{name: "negative defaults to 30s", in: -1 * time.Second, want: 30 * time.Second},
		{name: "below min clamps to 5s", in: 1 * time.Second, want: 5 * time.Second},
		{name: "exactly min stays", in: 5 * time.Second, want: 5 * time.Second},
		{name: "in range stays", in: 45 * time.Second, want: 45 * time.Second},
		{name: "exactly max stays", in: 10 * time.Minute, want: 10 * time.Minute},
		{name: "above max clamps to 10m", in: 1 * time.Hour, want: 10 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := VendorConfig{PollInterval: tt.in}
			validateVendorConfig(&cfg, &[]ClampAdjustment{})
			if cfg.PollInterval != tt.want {
				t.Errorf("PollInterval = %v, want %v", cfg.PollInterval, tt.want)
			}
		})
	}
}

func TestValidateNotificationsConfig_Clamps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		in            NotificationConfig
		wantPrune     time.Duration
		wantRetention time.Duration
		wantRecent    int
	}{
		{
			name:          "zero values fall back to defaults",
			in:            NotificationConfig{},
			wantPrune:     time.Hour,
			wantRetention: 30 * 24 * time.Hour,
			wantRecent:    20,
		},
		{
			name:          "in range stays",
			in:            NotificationConfig{PruneInterval: 2 * time.Hour, Retention: 240 * time.Hour, RecentLimit: 50},
			wantPrune:     2 * time.Hour,
			wantRetention: 240 * time.Hour,
			wantRecent:    50,
		},
		{
			name:          "below minimums clamp up",
			in:            NotificationConfig{PruneInterval: time.Second, Retention: time.Minute, RecentLimit: 0},
			wantPrune:     time.Minute,
			wantRetention: time.Hour,
			wantRecent:    20,
		},
		{
			name:          "above maximums clamp down",
			in:            NotificationConfig{PruneInterval: 48 * time.Hour, Retention: 10000 * time.Hour, RecentLimit: 1000},
			wantPrune:     24 * time.Hour,
			wantRetention: 365 * 24 * time.Hour,
			wantRecent:    100,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := tt.in
			validateNotificationsConfig(&cfg, &[]ClampAdjustment{})
			if cfg.PruneInterval != tt.wantPrune {
				t.Errorf("PruneInterval = %v, want %v", cfg.PruneInterval, tt.wantPrune)
			}
			if cfg.Retention != tt.wantRetention {
				t.Errorf("Retention = %v, want %v", cfg.Retention, tt.wantRetention)
			}
			if cfg.RecentLimit != tt.wantRecent {
				t.Errorf("RecentLimit = %d, want %d", cfg.RecentLimit, tt.wantRecent)
			}
		})
	}
}

func TestValidateCurrencyConfig_ClampsReapAfter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{name: "zero defaults to 30m", in: 0, want: 30 * time.Minute},
		{name: "negative defaults to 30m", in: -1 * time.Minute, want: 30 * time.Minute},
		{name: "below min clamps to 5m", in: 1 * time.Minute, want: 5 * time.Minute},
		{name: "exactly min stays", in: 5 * time.Minute, want: 5 * time.Minute},
		{name: "in range stays", in: 2 * time.Hour, want: 2 * time.Hour},
		{name: "exactly max stays", in: 24 * time.Hour, want: 24 * time.Hour},
		{name: "above max clamps to 24h", in: 48 * time.Hour, want: 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := CurrencyConfig{
				Cooldown:              5 * time.Minute,
				DepositPollInterval:   30 * time.Second,
				WithdrawDrainInterval: 30 * time.Second,
				ReapAfter:             tt.in,
				MaxZenyPerTx:          1_000_000,
				MaxCashpointPerTx:     1000,
			}
			validateCurrencyConfig(&cfg, &[]ClampAdjustment{})
			if cfg.ReapAfter != tt.want {
				t.Errorf("ReapAfter = %v, want %v", cfg.ReapAfter, tt.want)
			}
		})
	}
}

func TestValidateCurrencyConfig_PanicsOnNonPositiveMax(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		wantContain string
		zenyPerTx   int64
		cashPerTx   int
	}{
		{name: "zero zeny", zenyPerTx: 0, cashPerTx: 1000, wantContain: "Currency.MaxZenyPerTx must be > 0"},
		{name: "negative zeny", zenyPerTx: -1, cashPerTx: 1000, wantContain: "Currency.MaxZenyPerTx must be > 0"},
		{name: "zero cashpoint", zenyPerTx: 1_000_000, cashPerTx: 0, wantContain: "Currency.MaxCashpointPerTx must be > 0"},
		{name: "negative cashpoint", zenyPerTx: 1_000_000, cashPerTx: -1, wantContain: "Currency.MaxCashpointPerTx must be > 0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := CurrencyConfig{
				Cooldown:              5 * time.Minute,
				DepositPollInterval:   30 * time.Second,
				WithdrawDrainInterval: 30 * time.Second,
				ReapAfter:             30 * time.Minute,
				MaxZenyPerTx:          tt.zenyPerTx,
				MaxCashpointPerTx:     tt.cashPerTx,
			}
			msg := mustPanic(t, func() { validateCurrencyConfig(&cfg, &[]ClampAdjustment{}) })
			if !strings.Contains(msg, tt.wantContain) {
				t.Errorf("panic message = %q, want substring %q", msg, tt.wantContain)
			}
		})
	}
}

func TestValidatePurchasesConfig_EmptyIsDisabledNoPanic(t *testing.T) {
	t.Parallel()
	cfg := PurchasesConfig{}
	validatePurchasesConfig(&cfg)
}

func TestValidatePurchasesConfig_PanicsOnBadPackages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		wantContain string
		cfg         PurchasesConfig
	}{
		{
			name:        "bad currency",
			cfg:         PurchasesConfig{Currency: "US", Packages: []PackageConfig{{Key: "x", Name: "X", Price: 1, CashPoints: 1}}},
			wantContain: "Purchases.Currency",
		},
		{
			name:        "bad key",
			cfg:         PurchasesConfig{Currency: "USD", Packages: []PackageConfig{{Key: "Bad Key", Name: "X", Price: 1, CashPoints: 1}}},
			wantContain: "must match",
		},
		{
			name:        "duplicate key",
			cfg:         PurchasesConfig{Currency: "USD", Packages: []PackageConfig{{Key: "x", Name: "X", Price: 1, CashPoints: 1}, {Key: "x", Name: "Y", Price: 1, CashPoints: 1}}},
			wantContain: "duplicated",
		},
		{
			name:        "non-positive price",
			cfg:         PurchasesConfig{Currency: "USD", Packages: []PackageConfig{{Key: "x", Name: "X", Price: 0, CashPoints: 1}}},
			wantContain: "Price must be > 0",
		},
		{
			name:        "non-positive points",
			cfg:         PurchasesConfig{Currency: "USD", Packages: []PackageConfig{{Key: "x", Name: "X", Price: 1, CashPoints: 0}}},
			wantContain: "CashPoints must be > 0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := tt.cfg
			msg := mustPanic(t, func() { validatePurchasesConfig(&cfg) })
			if !strings.Contains(msg, tt.wantContain) {
				t.Errorf("panic = %q, want substring %q", msg, tt.wantContain)
			}
		})
	}
}

func TestLoadAppConfigFromDir_RecordsOutOfRangeClampsOnly(t *testing.T) {
	t.Parallel()

	files := validConfFiles()
	files["polling.yml"] = `Vendor:
  PollInterval: "1s"
Metrics:
  OnlinePollInterval: "1m"
  GeneralPollInterval: "1h"
  PeakWindows: ["daily", "weekly", "monthly", "all_time"]
Currency:
  Cooldown: "100ms"
  DepositPollInterval: "30s"
  WithdrawDrainInterval: "30s"
  MaxZenyPerTx: 1000000
  MaxCashpointPerTx: 1000
`
	dir := writeConfDir(t, files)

	cfg := loadAppConfigFromDir(dir)

	got := map[string]ClampAdjustment{}
	for _, adjustment := range cfg.ClampWarnings() {
		got[adjustment.Field] = adjustment
	}

	if _, ok := got["Vendor.PollInterval"]; !ok {
		t.Errorf("expected Vendor.PollInterval clamp recorded, got %+v", cfg.ClampWarnings())
	}
	if _, ok := got["Currency.Cooldown"]; !ok {
		t.Errorf("expected Currency.Cooldown clamp recorded, got %+v", cfg.ClampWarnings())
	}
	if adj, ok := got["Currency.Cooldown"]; ok {
		if adj.Given != 100*time.Millisecond || adj.Clamped != 1*time.Minute {
			t.Errorf("Currency.Cooldown adjustment = %+v, want given=100ms clamped=1m", adj)
		}
	}
	if _, ok := got["Currency.DepositPollInterval"]; ok {
		t.Errorf("in-range DepositPollInterval must not be recorded: %+v", got)
	}
	if _, ok := got["Metrics.OnlinePollInterval"]; ok {
		t.Errorf("in-range Metrics.OnlinePollInterval must not be recorded: %+v", got)
	}
}

func TestValidateRatesConfig_ClampsNonPositiveToDefault(t *testing.T) {
	t.Parallel()

	cfg := RatesConfig{
		ExpRate:         0,
		JobRate:         -50,
		DropRateCommon:  200,
		DropRateHeal:    100,
		DropRateUsable:  0,
		DropRateEquip:   1,
		DropRateCard:    -1,
		DropRateCardMVP: 500,
	}
	validateRatesConfig(&cfg)

	want := RatesConfig{
		ExpRate:         100,
		JobRate:         100,
		DropRateCommon:  200,
		DropRateHeal:    100,
		DropRateUsable:  100,
		DropRateEquip:   1,
		DropRateCard:    100,
		DropRateCardMVP: 500,
	}
	if cfg != want {
		t.Errorf("validateRatesConfig = %+v, want %+v", cfg, want)
	}
}

func TestValidateTheme_AcceptsValidNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "default theme", in: "default", want: "default"},
		{name: "empty falls back to default", in: "", want: "default"},
		{name: "lowercase letters", in: "sample", want: "sample"},
		{name: "lowercase with digits", in: "theme01", want: "theme01"},
		{name: "lowercase with underscore", in: "my_theme", want: "my_theme"},
		{name: "single letter", in: "x", want: "x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := GeneralConfig{Theme: tt.in}
			validateTheme(&cfg)
			if cfg.Theme != tt.want {
				t.Errorf("Theme = %q, want %q", cfg.Theme, tt.want)
			}
		})
	}
}

func TestValidateTheme_RejectsInvalidNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		in          string
		wantContain string
	}{
		{name: "uppercase letter", in: "Sample", wantContain: "must match"},
		{name: "all uppercase", in: "DEFAULT", wantContain: "must match"},
		{name: "space inside", in: "my theme", wantContain: "must match"},
		{name: "leading space", in: " sample", wantContain: "must match"},
		{name: "trailing space", in: "sample ", wantContain: "must match"},
		{name: "hyphen rejected", in: "my-theme", wantContain: "must match"},
		{name: "dot rejected", in: "my.theme", wantContain: "must match"},
		{name: "slash rejected", in: "my/theme", wantContain: "must match"},
		{name: "exclamation rejected", in: "sample!", wantContain: "must match"},
		{name: "shell injection semicolon", in: "default; rm -rf /", wantContain: "must match"},
		{name: "shell injection backtick", in: "default`whoami`", wantContain: "must match"},
		{name: "newline rejected", in: "default\n", wantContain: "must match"},
		{name: "leading digit rejected", in: "2024_summer", wantContain: "must match"},
		{name: "all digits rejected", in: "42", wantContain: "must match"},
		{name: "leading underscore rejected", in: "_theme", wantContain: "must match"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := GeneralConfig{Theme: tt.in}
			msg := mustPanic(t, func() { validateTheme(&cfg) })
			if !strings.Contains(msg, tt.wantContain) {
				t.Errorf("panic message = %q, want substring %q", msg, tt.wantContain)
			}
		})
	}
}

func TestLoadAppConfigFromDir_ReadsAllSections(t *testing.T) {
	t.Parallel()
	dir := writeConfDir(t, validConfFiles())

	cfg := loadAppConfigFromDir(dir)

	if cfg.General.ServerName != "Test Panel" {
		t.Errorf("General.ServerName = %q, want %q", cfg.General.ServerName, "Test Panel")
	}
	if !cfg.Auth.AllowTempBannedLogin {
		t.Errorf("Auth.AllowTempBannedLogin = false, want true")
	}
	if cfg.TTL.Session != 24*time.Hour {
		t.Errorf("TTL.Session = %v, want 24h", cfg.TTL.Session)
	}
	if cfg.Security.HSTSMaxAge != 604800 {
		t.Errorf("Security.HSTSMaxAge = %d, want 604800", cfg.Security.HSTSMaxAge)
	}
	if cfg.UserRoles["Moderator"] != 20 {
		t.Errorf("UserRoles.Moderator = %d, want 20", cfg.UserRoles["Moderator"])
	}
	if cfg.TicketLimits.MaxOpenPerPlayer != 5 {
		t.Errorf("TicketLimits.MaxOpenPerPlayer = %d, want 5", cfg.TicketLimits.MaxOpenPerPlayer)
	}
	if _, ok := cfg.NewsCategories["Announcement"]; !ok {
		t.Errorf("NewsCategories missing Announcement")
	}
	if len(cfg.ItemDB.YAML.Files) != 1 || cfg.ItemDB.YAML.Files[0] != "item_db.yml" {
		t.Errorf("ItemDB.YAML.Files = %v, want [item_db.yml]", cfg.ItemDB.YAML.Files)
	}
	if cfg.Vendor.PollInterval != 30*time.Second {
		t.Errorf("Vendor.PollInterval = %v, want 30s", cfg.Vendor.PollInterval)
	}
	if cfg.Mailer.FromAddress != "noreply@test.example" {
		t.Errorf("Mailer.FromAddress = %q, want %q", cfg.Mailer.FromAddress, "noreply@test.example")
	}
}

func TestLoadAppConfigFromDir_PanicsOnMissingFile(t *testing.T) {
	t.Parallel()

	names := []string{
		"app.yml", "auth.yml", "security.yml", "roles.yml",
		"tickets.yml", "news.yml", "datasources.yml", "polling.yml",
		"purchases.yml",
	}
	for _, missing := range names {
		t.Run("missing_"+missing, func(t *testing.T) {
			t.Parallel()
			files := validConfFiles()
			delete(files, missing)
			dir := writeConfDir(t, files)

			msg := mustPanic(t, func() { loadAppConfigFromDir(dir) })
			if !strings.Contains(msg, missing) {
				t.Errorf("panic message = %q, want substring %q", msg, missing)
			}
		})
	}
}

func TestLoadAppConfigFromDir_TolereratesEmptyFiles(t *testing.T) {
	t.Parallel()

	empty := map[string]string{
		"app.yml": "", "auth.yml": "", "security.yml": "",
		"roles.yml": "", "tickets.yml": "", "news.yml": "",
		"datasources.yml": "", "polling.yml": "", "purchases.yml": "",
		"apikeys.yml": "", "notifications.yml": "",
	}
	dir := writeConfDir(t, empty)

	cfg := loadAppConfigFromDir(dir)

	if cfg.General.ServerName != "Go Control Panel" {
		t.Errorf("General.ServerName default = %q, want %q", cfg.General.ServerName, "Go Control Panel")
	}
	if cfg.Mailer.FromAddress != "noreply@gocp.com" {
		t.Errorf("Mailer.FromAddress default = %q, want %q", cfg.Mailer.FromAddress, "noreply@gocp.com")
	}
	if cfg.UserRoles["Moderator"] != 20 {
		t.Errorf("UserRoles.Moderator default = %d, want 20", cfg.UserRoles["Moderator"])
	}
	if cfg.TTL.Session != 24*time.Hour {
		t.Errorf("TTL.Session default = %v, want 24h", cfg.TTL.Session)
	}
}

func TestLoadAppConfigFromDir_AppAndMailerTagsUnmarshal(t *testing.T) {
	t.Parallel()

	files := validConfFiles()
	files["app.yml"] = `App:
  ServerName: "Renamed Server"
  Timezone: "UTC"
  Theme: "default"
Mailer:
  FromAddress: "ops@renamed.test"
DefaultLocation:
  Map: "prontera"
  X: 156
  Y: 191
`
	dir := writeConfDir(t, files)

	cfg := loadAppConfigFromDir(dir)

	if cfg.General.ServerName != "Renamed Server" {
		t.Errorf("App: tag did not route to General field: ServerName = %q", cfg.General.ServerName)
	}
	if cfg.Mailer.FromAddress != "ops@renamed.test" {
		t.Errorf("Mailer: tag did not route to Mailer field: FromAddress = %q", cfg.Mailer.FromAddress)
	}
}
