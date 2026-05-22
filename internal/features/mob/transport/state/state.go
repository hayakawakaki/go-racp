package state

import (
	"fmt"

	"github.com/hayakawakaki/go-racp/internal/features/mob/app"
	"github.com/hayakawakaki/go-racp/internal/features/mob/domain"
)

type DetailState struct {
	Mob      *domain.Mob
	Stats    []LabeledRow
	Exp      []LabeledRow
	Drops    []DropRow
	MvpDrops []DropRow
}

type ListState struct {
	Query   string
	BaseURL string
	Page    app.MobPage
}

type LabeledRow struct {
	Label string
	Value string
}

type DropRow struct {
	Aegis      string
	Image      string
	ClientName string
	ItemID     int
	Slots      int
	Rate       int
}

func (r DropRow) DisplayName() string {
	name := r.ClientName
	if name == "" {
		name = r.Aegis
	}
	if r.Slots > 0 {
		name += fmt.Sprintf(" [%d]", r.Slots)
	}

	return name
}
