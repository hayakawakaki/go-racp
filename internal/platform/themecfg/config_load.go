package themecfg

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/hayakawakaki/go-racp/server/config"
)

const defaultThemeName = "default"

func LoadCfg(activeTheme string) error {
	root, err := config.ProjectRoot()
	if err != nil {
		return fmt.Errorf("locate project root: %w", err)
	}

	return loadCfgLayered(filepath.Join(root, "themes"), activeTheme)
}

func loadCfgLayered(themesDir, activeTheme string) error {
	Cfg = Config{}

	if err := overlayThemeConfig(themesDir, defaultThemeName); err != nil {
		return err
	}

	if activeTheme == defaultThemeName {
		return nil
	}

	return overlayThemeConfig(themesDir, activeTheme)
}

func overlayThemeConfig(themesDir, themeName string) error {
	path := filepath.Join(themesDir, themeName, "config.yml")

	data, err := os.ReadFile(path) //nolint:gosec // path joined from themes dir + theme name + fixed filename
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("read %s: %w", path, err)
	}

	if len(data) == 0 {
		return nil
	}

	if err := yaml.Unmarshal(data, &Cfg); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	return nil
}
