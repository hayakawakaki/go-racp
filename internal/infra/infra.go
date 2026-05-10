package infra

import (
	"database/sql"
	"log/slog"

	"github.com/hayakawakaki/go-racp/server/config"
	"github.com/wneessen/go-mail"
)

type Infra struct {
	MainDB *sql.DB
	LogDB  *sql.DB
	Logger *slog.Logger
	Mailer *mail.Client
	Config *config.Config
}
