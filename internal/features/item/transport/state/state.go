package state

import (
	"github.com/hayakawakaki/go-racp/internal/features/item/app"
	"github.com/hayakawakaki/go-racp/internal/features/item/domain"
	mobdomain "github.com/hayakawakaki/go-racp/internal/features/mob/domain"
)

type DetailState struct {
	Item             *domain.Item
	Stats            []LabeledRow
	DescriptionLines []string
	DroppedBy        []mobdomain.DropOf
}

type ListState struct {
	Query   string
	Type    string
	BaseURL string
	Page    app.ItemPage
}

type LabeledRow struct {
	Label string
	Value string
}
