package app

import (
	"github.com/hayakawakaki/go-racp/internal/features/mob/domain"
)

const DefaultPerPage = 20

type ListQuery struct {
	Query   string
	Page    int
	PerPage int
}

type MobPage struct {
	Mobs       []*domain.Mob
	Total      int
	Page       int
	PerPage    int
	TotalPages int
}

type ServiceStatus struct {
	LastReload string
	LastError  string
	MobsLoaded int
}

type DropDTO struct {
	ItemAegis         string `json:"item_aegis"`
	RandomOptionGroup string `json:"random_option_group,omitempty"`
	Rate              int    `json:"rate"`
	StealProtected    bool   `json:"steal_protected,omitempty"`
}

//nolint:govet // DTO ordered for JSON readability
type MobDTO struct {
	AegisName    string    `json:"aegis_name"`
	Name         string    `json:"name"`
	JapaneseName string    `json:"japanese_name,omitempty"`
	Sprite       string    `json:"sprite"`
	Race         string    `json:"race"`
	Element      string    `json:"element"`
	Size         string    `json:"size"`
	Modes        []string  `json:"modes,omitempty"`
	Drops        []DropDTO `json:"drops,omitempty"`
	MvpDrops     []DropDTO `json:"mvp_drops,omitempty"`
	ID           int       `json:"id"`
	Level        int       `json:"level"`
	HP           int       `json:"hp"`
	BaseExp      int       `json:"base_exp"`
	JobExp       int       `json:"job_exp"`
	MvpExp       int       `json:"mvp_exp,omitempty"`
	IsMVP        bool      `json:"is_mvp,omitempty"`
}

func toDropDTOs(drops []domain.MobDrop) []DropDTO {
	if len(drops) == 0 {
		return nil
	}
	out := make([]DropDTO, 0, len(drops))
	for _, drop := range drops {
		out = append(out, DropDTO{
			ItemAegis:         drop.ItemAegis,
			RandomOptionGroup: drop.RandomOptionGroup,
			Rate:              drop.Rate,
			StealProtected:    drop.StealProtected,
		})
	}

	return out
}

func ToDTO(mob *domain.Mob) MobDTO {
	if mob == nil {
		return MobDTO{}
	}

	return MobDTO{
		ID:           mob.ID,
		AegisName:    mob.AegisName,
		Name:         mob.Name,
		JapaneseName: mob.JapaneseName,
		Sprite:       domain.ResolveSprite(mob.AegisName),
		Level:        mob.Level,
		HP:           mob.HP,
		BaseExp:      mob.BaseExp,
		JobExp:       mob.JobExp,
		MvpExp:       mob.MvpExp,
		Race:         mob.Race.Display(),
		Element:      mob.Element.Display(),
		Size:         mob.Size.Display(),
		Modes:        mob.Modes.Display(),
		Drops:        toDropDTOs(mob.Drops),
		MvpDrops:     toDropDTOs(mob.MvpDrops),
		IsMVP:        mob.IsMVP(),
	}
}
