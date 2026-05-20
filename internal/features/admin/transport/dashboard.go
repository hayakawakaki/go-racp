package transport

import (
	"encoding/json"

	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
)

type dashboardState struct {
	PeakTable peakTable              `json:"-"`
	General   domain.GeneralSnapshot `json:"general"`
	Online    domain.OnlineSnapshot  `json:"online"`
}

type alpineSeed struct {
	General domain.GeneralSnapshot `json:"general"`
	Online  domain.OnlineSnapshot  `json:"online"`
}

type peakTable struct {
	Rows      []peakTableRow
	HasUnique bool
}

type peakTableRow struct {
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

func dashboardAlpineState(state dashboardState) string {
	encoded, err := json.Marshal(alpineSeed{Online: state.Online, General: state.General})
	if err != nil {
		return "{ online: {}, general: {} }"
	}
	return "Object.assign(" + string(encoded) + ", { startPolling() { const onlineTick = async () => { const o = await fetch('/api/v1/metrics/online').then(r => r.json()); this.online = o; }; const generalTick = async () => { const g = await fetch('/api/v1/metrics/general').then(r => r.json()); this.general = g; }; setInterval(() => onlineTick().catch(() => {}), 60000); setInterval(() => generalTick().catch(() => {}), 3600000); } })"
}

func buildPeakTable(peaks []domain.PeakRow) peakTable {
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

	table := peakTable{}
	for _, w := range windowOrder {
		bucket, has := byWindow[w]
		if !has {
			continue
		}
		_, hasUnique := bucket[domain.MetricOnlineUnique]
		if hasUnique {
			table.HasUnique = true
		}
		table.Rows = append(table.Rows, peakTableRow{
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
