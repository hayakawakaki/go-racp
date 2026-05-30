package helpers

import (
	"net/url"
	"strconv"
	"time"
)

func FormatTime(t time.Time, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}

	return t.In(loc).Format("2006-01-02 15:04")
}

func FormatSentTime(t *time.Time, loc *time.Location) string {
	if t == nil {
		return "not sent"
	}

	return FormatTime(*t, loc)
}

func FormatDeliveredTime(t *time.Time, loc *time.Location) string {
	if t == nil {
		return "not delivered"
	}

	return FormatTime(*t, loc)
}

func WithdrawStatusLabel(status int) string {
	switch status {
	case 3:
		return "Delivered"
	case 2:
		return "Sent"
	default:
		return "Pending"
	}
}

func HistoryHref(basePath, primaryKey, primaryValue, otherKey string, otherPage int) string {
	values := url.Values{}
	values.Set(primaryKey, primaryValue)
	if otherPage > 1 {
		values.Set(otherKey, strconv.Itoa(otherPage))
	}

	return basePath + "?" + values.Encode()
}
