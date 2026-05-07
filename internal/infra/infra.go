package infra

import (
	"database/sql"
	"log/slog"
)

type Infra struct {
	MainDB *sql.DB
	LogDB  *sql.DB
	Logger *slog.Logger
}
