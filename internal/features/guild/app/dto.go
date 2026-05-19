package app

import "github.com/hayakawakaki/go-racp/internal/features/guild/domain"

type ListQuery = domain.ListQuery

type GuildPage = domain.GuildPage

type GuildDetail struct {
	Guild   *domain.Guild
	Members []domain.Member
}
