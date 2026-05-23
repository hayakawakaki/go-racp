package state

import (
	"github.com/hayakawakaki/go-racp/internal/features/stall/domain"
)

type ListState struct {
	Type    string
	BaseURL string
	Page    domain.Page
	ItemID  int
}

type StallItemRow struct {
	ItemName string
	Aegis    string
	ItemID   int
	Amount   int
	Price    int
}

type StallState struct {
	Items  []StallItemRow
	Vendor domain.Vendor
}
