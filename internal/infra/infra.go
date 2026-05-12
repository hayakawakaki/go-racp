package infra

import (
	"database/sql"
	"log/slog"

	"github.com/hayakawakaki/go-racp/internal/actiontoken"
	"github.com/hayakawakaki/go-racp/internal/infra/mailer"
	"github.com/hayakawakaki/go-racp/server/config"
)

type Infra struct {
	MainDB       *sql.DB
	LogDB        *sql.DB
	Logger       *slog.Logger
	Mailer       *mailer.SMTPMailer
	TokenManager *actiontoken.Manager
	Config       *config.Config
}
