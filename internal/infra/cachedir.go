package infra

import (
	"log/slog"
	"path/filepath"

	"github.com/hayakawakaki/go-racp/server/config"
)

const DevCacheSubdir = "tmp"

func DevCacheDir(mode string, logger *slog.Logger) string {
	if mode != "development" {
		return ""
	}
	root, err := config.ProjectRoot()
	if err != nil {
		logger.Warn("dev cache disabled, project root not found", "err", err)
		return ""
	}

	return filepath.Join(root, DevCacheSubdir)
}
