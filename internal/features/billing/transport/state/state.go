package state

import "github.com/hayakawakaki/go-racp/internal/features/billing/domain"

type StoreState struct {
	Currency  string
	Notice    string
	Packages  []domain.Package
	Available bool
}
