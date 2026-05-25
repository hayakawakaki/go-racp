package themecfg

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/hayakawakaki/go-racp/server/config"
)

func LoadCfg(themeName string) error {
	root, err := config.ProjectRoot()
	if err != nil {
		return fmt.Errorf("locate project root: %w", err)
	}

	return loadCfgFromDir(filepath.Join(root, "themes", themeName))
}

func loadCfgFromDir(dir string) error {
	path := filepath.Join(dir, "config.yml")

	data, err := os.ReadFile(path) //nolint:gosec // path joined from caller-supplied themes dir and fixed filename
	if err != nil {
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
