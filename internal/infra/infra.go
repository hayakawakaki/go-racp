package infra

import (
	"database/sql"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/actiontoken"
	"github.com/hayakawakaki/go-racp/internal/infra/mailer"
	"github.com/hayakawakaki/go-racp/server/config"
)

type Infra struct {
	MainDB       *sql.DB
	LogDB        *sql.DB
	DB           *pgxpool.Pool
	Logger       *slog.Logger
	Mailer       *mailer.SMTPMailer
	TokenManager *actiontoken.Manager
	Config       *config.Config
	Roles        domain.RoleResolver
}
