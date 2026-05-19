package domain

import "context"

type Guild struct {
	Name         string
	MasterName   string
	ID           int
	MasterCharID int
	GuildLevel   int
	MaxMember    int
}

type Member struct {
	Name         string
	PositionName string
	CharID       int
	Position     int
}

type ListQuery struct {
	Query   string
	Page    int
	PerPage int
}

type GuildPage struct {
	Guilds     []Guild
	Total      int
	Page       int
	PerPage    int
	TotalPages int
}

type Repository interface {
	List(ctx context.Context, q ListQuery) (GuildPage, error)
	GetByID(ctx context.Context, id int) (*Guild, error)
	ListMembers(ctx context.Context, guildID int) ([]Member, error)
	GetEmblem(ctx context.Context, guildID int) (data []byte, mime string, err error)
}
