package domain

import "time"

const maxBanDays = 10 * 365

type BanDuration struct {
	Duration  time.Duration
	Permanent bool
}

func ParseBanDays(days int) (BanDuration, error) {
	if days <= 0 || days > maxBanDays {
		return BanDuration{}, ErrInvalidDuration
	}

	return BanDuration{Duration: time.Duration(days) * 24 * time.Hour}, nil
}
