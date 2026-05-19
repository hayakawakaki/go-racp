package infra

import (
	"database/sql"
	"log/slog"

	accdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/jackc/pgx/v5/pgxpool"

	actiontokenapp "github.com/hayakawakaki/go-racp/internal/actiontoken/app"
	"github.com/hayakawakaki/go-racp/internal/infra/mailer"
	"github.com/hayakawakaki/go-racp/server/config"
)

type Infra struct {
	MainDB       *sql.DB
	LogDB        *sql.DB
	DB           *pgxpool.Pool
	Logger       *slog.Logger
	Mailer       *mailer.SMTPMailer
	TokenManager *actiontokenapp.Manager
	Config       *config.Config
	Roles        accdomain.RoleResolver
}
