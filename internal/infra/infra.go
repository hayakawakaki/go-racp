package infra

import (
	"database/sql"
	"log/slog"

	accdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
	actiontokenapp "github.com/hayakawakaki/go-racp/internal/platform/actiontoken/app"
	"github.com/jackc/pgx/v5/pgxpool"

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
