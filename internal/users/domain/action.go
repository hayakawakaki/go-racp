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

const banCeiling = 10 * 365 * 24 * time.Hour

type BanDuration struct {
	Duration  time.Duration
	Permanent bool
}

func ParseBanPreset(preset string) (BanDuration, error) {
	switch preset {
	case "1h":
		return BanDuration{Duration: time.Hour}, nil
	case "1d":
		return BanDuration{Duration: 24 * time.Hour}, nil
	case "7d":
		return BanDuration{Duration: 7 * 24 * time.Hour}, nil
	case "30d":
		return BanDuration{Duration: 30 * 24 * time.Hour}, nil
	case "perm":
		return BanDuration{Permanent: true}, nil
	default:
		return BanDuration{}, ErrInvalidDuration
	}
}

func ParseBanCustom(value int, unit string) (BanDuration, error) {
	if value <= 0 {
		return BanDuration{}, ErrInvalidDuration
	}
	var step time.Duration
	switch unit {
	case "hours":
		step = time.Hour
	case "days":
		step = 24 * time.Hour
	default:
		return BanDuration{}, ErrInvalidDuration
	}
	total := time.Duration(value) * step
	if total > banCeiling {
		return BanDuration{}, ErrInvalidDuration
	}

	return BanDuration{Duration: total}, nil
}
