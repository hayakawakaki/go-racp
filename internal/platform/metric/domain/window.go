package domain

import "time"

var allTimeSentinel = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

func WindowKey(w Window, now time.Time) time.Time {
	switch w {
	case WindowDaily:
		y, m, d := now.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	case WindowWeekly:
		y, m, d := now.Date()
		midnight := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
		weekday := int(midnight.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return midnight.AddDate(0, 0, -(weekday - 1))
	case WindowMonthly:
		y, m, _ := now.Date()
		return time.Date(y, m, 1, 0, 0, 0, 0, now.Location())
	case WindowAllTime:
		return allTimeSentinel
	}
	return allTimeSentinel
}
