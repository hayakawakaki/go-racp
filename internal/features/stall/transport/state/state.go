package state

import (
	"github.com/hayakawakaki/go-racp/internal/features/stall/domain"
)

type ListState struct {
	BaseURL     string
	BuyingPage  domain.Page
	SellingPage domain.Page
	ItemID      int
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
