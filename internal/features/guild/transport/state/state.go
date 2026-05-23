package state

import (
	"github.com/hayakawakaki/go-racp/internal/features/guild/app"
)

type ListState struct {
	Query   string
	BaseURL string
	Page    app.GuildPage
}

type DetailState struct {
	Detail app.GuildDetail
}
