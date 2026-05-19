package self

import "time"

const (
	StateActive      = 0
	StateUnverified  = 1
	StatePermaBanned = 5
)

type Tier int

const (
	TierActive Tier = iota
	TierUnverified
	TierTempBanned
	TierPermaBanned
	TierDeleted
)

func (t Tier) String() string {
	switch t {
	case TierActive:
		return "active"
	case TierUnverified:
		return "unverified"
	case TierTempBanned:
		return "temp_banned"
	case TierPermaBanned:
		return "perma_banned"
	case TierDeleted:
		return "deleted"
	default:
		return "unknown"
	}
}

func ClassifyTier(state int, unbanTime, now time.Time) Tier {
	switch state {
	case StateUnverified:
		return TierUnverified
	case StatePermaBanned:
		return TierPermaBanned
	case StateActive:
		if !unbanTime.IsZero() && unbanTime.After(now) {
			return TierTempBanned
		}
		return TierActive
	default:
		return TierActive
	}
}
