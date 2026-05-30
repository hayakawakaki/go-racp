package state

import (
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
)

type StoreState struct {
	Currency  string
	Notice    string
	Packages  []domain.Package
	Available bool
}

type PurchaseHistoryState struct {
	Location  *time.Location
	Currency  string
	Purchases []domain.Purchase
}
