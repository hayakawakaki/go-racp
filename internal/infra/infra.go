package infra

import (
	"database/sql"
	"log/slog"
	"sync"

	"github.com/hayakawakaki/go-racp/internal/accountchange"
	"github.com/hayakawakaki/go-racp/internal/actiontoken"
	"github.com/hayakawakaki/go-racp/internal/infra/mailer"
	"github.com/hayakawakaki/go-racp/server/config"
)

type Infra struct {
	MainDB        *sql.DB
	LogDB         *sql.DB
	Logger        *slog.Logger
	Mailer        *mailer.SMTPMailer
	TokenManager  *actiontoken.Manager
	ChangeLog     accountchange.Repository
	EmailUniqueMu *sync.Mutex
	Config        *config.Config
}
