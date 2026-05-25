package themecfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeThemeConfig(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "themes", "default")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return root
}

func TestLoadCfgFromDir_ReadsFile(t *testing.T) {
	root := writeThemeConfig(t, `Branding:
  Logo: "/logo.svg"
  Discord: "https://discord.gg/test"
Navbar:
  Items:
    - Label: "Home"
      Href: "/"
      Icon: "home"
      Children: []
`)

	Cfg = Config{}
	t.Cleanup(func() { Cfg = Config{} })

	if err := loadCfgFromDir(filepath.Join(root, "themes", "default")); err != nil {
		t.Fatalf("loadCfgFromDir: %v", err)
	}
	if Cfg.Branding.Logo != "/logo.svg" {
		t.Errorf("Cfg.Branding.Logo = %q, want %q", Cfg.Branding.Logo, "/logo.svg")
	}
	if Cfg.Branding.Discord != "https://discord.gg/test" {
		t.Errorf("Cfg.Branding.Discord = %q", Cfg.Branding.Discord)
	}
	if len(Cfg.Navbar.Items) != 1 || Cfg.Navbar.Items[0].Label != "Home" {
		t.Errorf("Cfg.Navbar.Items = %#v, want one Home item", Cfg.Navbar.Items)
	}
}

func TestLoadCfgFromDir_MissingFilePanics(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := loadCfgFromDir(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "config.yml") {
		t.Errorf("error = %q, want substring %q", err.Error(), "config.yml")
	}
}

func TestLoadCfgFromDir_EmptyFileTolerated(t *testing.T) {
	root := writeThemeConfig(t, ``)

	Cfg = Config{}
	t.Cleanup(func() { Cfg = Config{} })

	if err := loadCfgFromDir(filepath.Join(root, "themes", "default")); err != nil {
		t.Fatalf("loadCfgFromDir: %v", err)
	}
}

func TestLoadCfgFromDir_MalformedYAMLErrors(t *testing.T) {
	t.Parallel()

	root := writeThemeConfig(t, `Branding:
  Logo: "unterminated
`)

	err := loadCfgFromDir(filepath.Join(root, "themes", "default"))
	if err == nil {
		t.Fatal("expected error on malformed yaml, got nil")
	}
}
