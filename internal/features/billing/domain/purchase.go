package domain

import (
	"context"
	"net/url"
	"slices"
	"time"
)

const (
	StatusPending   = 1
	StatusCompleted = 2
	StatusDisputed  = 3
	StatusRefunded  = 4
	StatusFailed    = 5
)

func StatusLabel(status int) string {
	switch status {
	case StatusPending:
		return "Pending"
	case StatusCompleted:
		return "Completed"
	case StatusDisputed:
		return "Disputed"
	case StatusRefunded:
		return "Refunded"
	case StatusFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

type Package struct {
	Key        string
	Name       string
	Currency   string
	Price      int64
	CashPoints int
}

type Catalog struct {
	byKey map[string]Package
	order []Package
}

func NewCatalog(packages []Package) Catalog {
	byKey := make(map[string]Package, len(packages))
	for _, pkg := range packages {
		byKey[pkg.Key] = pkg
	}

	return Catalog{order: packages, byKey: byKey}
}

func (c Catalog) List() []Package { return slices.Clone(c.order) }

func (c Catalog) Lookup(key string) (Package, bool) {
	pkg, ok := c.byKey[key]
	return pkg, ok
}

type Purchase struct {
	CreatedAt         time.Time
	CompletedAt       *time.Time
	DisputedAt        *time.Time
	PackageKey        string
	Provider          string
	ProviderRef       string
	ProviderPaymentID string
	Currency          string
	ID                int64
	Amount            int64
	AccountID         int
	CashPoints        int
	Status            int
}

type PurchaseFilter struct {
	From      *time.Time
	To        *time.Time
	Provider  string
	Status    int
	AccountID int
}

type EarningsSummary struct {
	Today   int64
	Week    int64
	Month   int64
	AllTime int64
}

type CheckoutRequest struct {
	PackageKey  string
	Description string
	Currency    string
	SuccessURL  string
	CancelURL   string
	PurchaseID  int64
	Amount      int64
}

type CheckoutResult struct {
	RedirectURL string
	Reference   string
}

type CheckoutConfirmation struct {
	PurchaseID int64
	Paid       bool
}

type Provider interface {
	Name() string
	CreateCheckout(ctx context.Context, request CheckoutRequest) (CheckoutResult, error)
	RetrieveCheckout(ctx context.Context, values url.Values) (CheckoutConfirmation, error)
}

type CaptureOutcome struct {
	PaymentID string
	Completed bool
}

type Capturer interface {
	Capture(ctx context.Context, reference string) (CaptureOutcome, error)
}
