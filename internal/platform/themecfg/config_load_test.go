package themecfg

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTwoThemeConfigs(t *testing.T, defaultBody, activeBody, activeName string) string {
	t.Helper()
	root := t.TempDir()

	defaultDir := filepath.Join(root, "themes", "default")
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		t.Fatalf("mkdir default: %v", err)
	}
	if err := os.WriteFile(filepath.Join(defaultDir, "config.yml"), []byte(defaultBody), 0o600); err != nil {
		t.Fatalf("write default config: %v", err)
	}

	activeDir := filepath.Join(root, "themes", activeName)
	if err := os.MkdirAll(activeDir, 0o755); err != nil {
		t.Fatalf("mkdir active: %v", err)
	}
	if err := os.WriteFile(filepath.Join(activeDir, "config.yml"), []byte(activeBody), 0o600); err != nil {
		t.Fatalf("write active config: %v", err)
	}

	return root
}

func writeSingleThemeConfig(t *testing.T, themeName, body string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "themes", themeName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return filepath.Join(root, "themes")
}

func TestLoadCfgLayered_OverlaysActiveOnDefault(t *testing.T) {
	root := writeTwoThemeConfigs(t, `Branding:
  Logo: "/default-logo.svg"
  Discord: "https://discord.gg/default"
`, `Branding:
  Logo: "/sample-logo.svg"
`, "sample")

	Cfg = Config{}
	t.Cleanup(func() { Cfg = Config{} })

	if err := loadCfgLayered(filepath.Join(root, "themes"), "sample"); err != nil {
		t.Fatalf("loadCfgLayered: %v", err)
	}
	if Cfg.Branding.Logo != "/sample-logo.svg" {
		t.Errorf("Logo = %q, want %q (active should override)", Cfg.Branding.Logo, "/sample-logo.svg")
	}
	if Cfg.Branding.Discord != "https://discord.gg/default" {
		t.Errorf("Discord = %q, want %q (default should fill missing field)", Cfg.Branding.Discord, "https://discord.gg/default")
	}
}

func TestLoadCfgLayered_DefaultThemeOnlyLoadsDefault(t *testing.T) {
	root := writeTwoThemeConfigs(t, `Branding:
  Logo: "/default-logo.svg"
`, `Branding:
  Logo: "/sample-logo.svg"
`, "sample")

	Cfg = Config{}
	t.Cleanup(func() { Cfg = Config{} })

	if err := loadCfgLayered(filepath.Join(root, "themes"), "default"); err != nil {
		t.Fatalf("loadCfgLayered: %v", err)
	}
	if Cfg.Branding.Logo != "/default-logo.svg" {
		t.Errorf("Logo = %q, want %q (active=default should not load sample)", Cfg.Branding.Logo, "/default-logo.svg")
	}
}

func TestLoadCfgLayered_MissingActiveTolerated(t *testing.T) {
	root := writeTwoThemeConfigs(t, `Branding:
  Logo: "/default-logo.svg"
`, `Branding:
  Logo: "/sample-logo.svg"
`, "sample")

	if err := os.Remove(filepath.Join(root, "themes", "sample", "config.yml")); err != nil {
		t.Fatalf("remove active config: %v", err)
	}

	Cfg = Config{}
	t.Cleanup(func() { Cfg = Config{} })

	if err := loadCfgLayered(filepath.Join(root, "themes"), "sample"); err != nil {
		t.Fatalf("loadCfgLayered: %v", err)
	}
	if Cfg.Branding.Logo != "/default-logo.svg" {
		t.Errorf("Logo = %q, want default value (active config missing should fall back)", Cfg.Branding.Logo)
	}
}

func TestLoadCfgLayered_ReadsDefault(t *testing.T) {
	themes := writeSingleThemeConfig(t, "default", `Branding:
  Logo: "/logo.svg"
  Discord: "https://discord.gg/test"
`)

	Cfg = Config{}
	t.Cleanup(func() { Cfg = Config{} })

	if err := loadCfgLayered(themes, "default"); err != nil {
		t.Fatalf("loadCfgLayered: %v", err)
	}
	if Cfg.Branding.Logo != "/logo.svg" {
		t.Errorf("Logo = %q", Cfg.Branding.Logo)
	}
}

func TestLoadCfgLayered_MissingDefaultTolerated(t *testing.T) {
	themes := t.TempDir()

	Cfg = Config{Branding: Branding{Logo: "stale"}}
	t.Cleanup(func() { Cfg = Config{} })

	if err := loadCfgLayered(themes, "default"); err != nil {
		t.Fatalf("loadCfgLayered: %v", err)
	}
	if Cfg.Branding.Logo != "" {
		t.Errorf("Cfg.Branding.Logo = %q, want zero (loader should reset)", Cfg.Branding.Logo)
	}
}

func TestLoadCfgLayered_EmptyDefaultFileTolerated(t *testing.T) {
	themes := writeSingleThemeConfig(t, "default", ``)

	Cfg = Config{}
	t.Cleanup(func() { Cfg = Config{} })

	if err := loadCfgLayered(themes, "default"); err != nil {
		t.Fatalf("loadCfgLayered: %v", err)
	}
}

func TestLoadCfgLayered_MalformedDefaultErrors(t *testing.T) {
	themes := writeSingleThemeConfig(t, "default", `Branding:
  Logo: "unterminated
`)

	err := loadCfgLayered(themes, "default")
	if err == nil {
		t.Fatal("expected error on malformed yaml, got nil")
	}
}
