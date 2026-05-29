package state

import (
	"time"

	currency "github.com/hayakawakaki/go-racp/internal/features/account/app/currency"
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
	Deposits        currency.DepositPage
	Withdraws       currency.WithdrawHistoryPage
	TotalsFailed    bool
	DepositsFailed  bool
	WithdrawsFailed bool
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
