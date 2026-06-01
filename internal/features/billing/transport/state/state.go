package state

import (
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
)

type PaymentMethod struct {
	Key     string
	Label   string
	Enabled bool
	Checked bool
}

type StoreState struct {
	Currency     string
	Notice       string
	Purchased    *domain.Package
	Packages     []domain.Package
	Methods      []PaymentMethod
	Available    bool
	NotCompleted bool
}

type PurchaseHistoryState struct {
	Location  *time.Location
	Currency  string
	Purchases []domain.Purchase
}
