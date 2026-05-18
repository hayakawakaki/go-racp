package domain

import "time"

type ActionKind string

const (
	ActionBan     ActionKind = "ban"
	ActionUnban   ActionKind = "unban"
	ActionSetRole ActionKind = "set_role"
)

type Action struct {
	At           time.Time
	Kind         ActionKind
	Reason       string
	BeforeValue  string
	AfterValue   string
	ID           int64
	ActorUserID  int
	TargetUserID int
}

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
