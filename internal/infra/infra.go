// Package infra defines shared infrastructure types that are constructed once
// at startup and passed to every plugin via plugin.MountAll.
package infra

import (
	"database/sql"
	"log/slog"
)

// Infra bundles the cross-cutting infrastructure dependencies that plugins
// require. It is created by server.Start and handed to each plugin's Mount
// function.
type Infra struct {
	// MainDB is the primary application database connection.
	MainDB *sql.DB

	// LogDB is the dedicated logging database connection.
	LogDB *sql.DB

	// Logger is the structured logger shared across all plugins.
	Logger *slog.Logger
}
