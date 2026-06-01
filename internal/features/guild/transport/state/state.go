package state

import (
	"github.com/hayakawakaki/go-racp/internal/features/guild/app"
	"github.com/hayakawakaki/go-racp/internal/features/guild/domain"
)

type ListState struct {
	Query   string
	BaseURL string
	Page    app.GuildPage
}

type DetailState struct {
	Detail     app.GuildDetail
	Members    []domain.Member
	Page       int
	TotalPages int
}
