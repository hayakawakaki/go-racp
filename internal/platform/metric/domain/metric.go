package domain

import "time"

type MetricName string

const (
	MetricOnlineTotal     MetricName = "online_total"
	MetricOnlineNonVendor MetricName = "online_non_vendor"
	MetricOnlineVendor    MetricName = "online_vendor"
	MetricOnlineUnique    MetricName = "online_unique"
)

type Window string

const (
	WindowDaily   Window = "daily"
	WindowWeekly  Window = "weekly"
	WindowMonthly Window = "monthly"
	WindowAllTime Window = "all_time"
)

type OnlineSnapshot struct {
	UpdatedAt time.Time `json:"updated_at"`
	Total     int       `json:"total"`
	Vendor    int       `json:"vendor"`
	NonVendor int       `json:"non_vendor"`
	Unique    int       `json:"unique"`
	HasUnique bool      `json:"has_unique"`
}

type PeakRow struct {
	OccurredAt time.Time  `json:"occurred_at"`
	WindowKey  time.Time  `json:"window_key"`
	Metric     MetricName `json:"metric"`
	Window     Window     `json:"window"`
	Value      int        `json:"value"`
}

type GeneralSnapshot struct {
	CapturedAt      time.Time `json:"captured_at"`
	TotalAccounts   int       `json:"total_accounts"`
	TotalCharacters int       `json:"total_characters"`
	TotalGuilds     int       `json:"total_guilds"`
}
