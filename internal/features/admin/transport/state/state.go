package state

import (
	itemapp "github.com/hayakawakaki/go-racp/internal/features/item/app"
	mobapp "github.com/hayakawakaki/go-racp/internal/features/mob/app"
	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
)

type DatabaseState struct {
	Item itemapp.ServiceStatus
	Mob  mobapp.ServiceStatus
}

type DashboardState struct {
	PeakTable PeakTable              `json:"-"`
	General   domain.GeneralSnapshot `json:"general"`
	Online    domain.OnlineSnapshot  `json:"online"`
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
	byWindow := map[domain.Window]map[domain.MetricName]int{}
	for _, p := range peaks {
		bucket, ok := byWindow[p.Window]
		if !ok {
			bucket = map[domain.MetricName]int{}
			byWindow[p.Window] = bucket
		}
		bucket[p.Metric] = p.Value
	}

	table := PeakTable{}
	for _, w := range windowOrder {
		bucket, has := byWindow[w]
		if !has {
			continue
		}
		_, hasUnique := bucket[domain.MetricOnlineUnique]
		if hasUnique {
			table.HasUnique = true
		}
		table.Rows = append(table.Rows, PeakTableRow{
			Window:    windowLabel(w),
			Total:     bucket[domain.MetricOnlineTotal],
			NonVendor: bucket[domain.MetricOnlineNonVendor],
			Vendor:    bucket[domain.MetricOnlineVendor],
			Unique:    bucket[domain.MetricOnlineUnique],
			HasUnique: hasUnique,
		})
	}
	return table
}
