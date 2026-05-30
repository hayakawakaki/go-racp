package state

import (
	"time"

	currency "github.com/hayakawakaki/go-racp/internal/features/account/app/currency"
	billingdomain "github.com/hayakawakaki/go-racp/internal/features/billing/domain"
	itemapp "github.com/hayakawakaki/go-racp/internal/features/item/app"
	mobapp "github.com/hayakawakaki/go-racp/internal/features/mob/app"
	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
)

type DatabaseState struct {
	Item itemapp.ServiceStatus
	Mob  mobapp.ServiceStatus
}

type DashboardState struct {
	PeakTable   PeakTable              `json:"-"`
	General     domain.GeneralSnapshot `json:"general"`
	Online      domain.OnlineSnapshot  `json:"online"`
	PeaksFailed bool                   `json:"-"`
}

//nolint:govet // grouped for readability over a few bytes of padding
type EconomyState struct {
	Location        *time.Location
	Totals          currency.TotalsDTO
	Earnings        billingdomain.EarningsSummary
	Deposits        currency.DepositPage
	Withdraws       currency.WithdrawHistoryPage
	Stuck           []currency.AdminWithdrawDTO
	Purchases       PurchasePage
	TotalsFailed    bool
	DepositsFailed  bool
	WithdrawsFailed bool
	StuckFailed     bool
	EarningsFailed  bool
	PurchasesFailed bool
}

type PurchaseFilterForm struct {
	Status   string
	Account  string
	Provider string
	From     string
	To       string
}

type PurchaseRow struct {
	Email    string
	Purchase billingdomain.Purchase
}

//nolint:govet // grouped for readability over a few bytes of padding
type PurchasePage struct {
	Rows        []PurchaseRow
	Form        PurchaseFilterForm
	HrefPattern string
	Page        int
	TotalPages  int
	Total       int
}

type PeakTable struct {
	Rows      []PeakTableRow
	HasUnique bool
}

type PeakTableRow struct {
	Window    string
	Total     int
	NonVendor int
	Vendor    int
	Unique    int
	HasUnique bool
}

var windowLabels = map[domain.Window]string{
	domain.WindowDaily:   "Daily",
	domain.WindowWeekly:  "Weekly",
	domain.WindowMonthly: "Monthly",
	domain.WindowAllTime: "All Time",
}

func windowLabel(w domain.Window) string {
	if label, ok := windowLabels[w]; ok {
		return label
	}
	return string(w)
}

func BuildPeakTable(peaks []domain.PeakRow) PeakTable {
	windowOrder := []domain.Window{
		domain.WindowDaily, domain.WindowWeekly, domain.WindowMonthly, domain.WindowAllTime,
	}
	set := domain.NewPeakSet(peaks)

	table := PeakTable{}
	for _, w := range windowOrder {
		if !set.HasWindow(w) {
			continue
		}
		hasUnique := set.HasMetric(w, domain.MetricOnlineUnique)
		if hasUnique {
			table.HasUnique = true
		}
		table.Rows = append(table.Rows, PeakTableRow{
			Window:    windowLabel(w),
			Total:     set.Value(w, domain.MetricOnlineTotal),
			NonVendor: set.Value(w, domain.MetricOnlineNonVendor),
			Vendor:    set.Value(w, domain.MetricOnlineVendor),
			Unique:    set.Value(w, domain.MetricOnlineUnique),
			HasUnique: hasUnique,
		})
	}
	return table
}
