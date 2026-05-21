package domain

import "time"

type LockoutPolicy struct {
	Window      time.Duration
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	Threshold   int
}

func DefaultLockoutPolicy() LockoutPolicy {
	return LockoutPolicy{
		Window:      15 * time.Minute,
		Threshold:   5,
		BaseBackoff: 30 * time.Second,
		MaxBackoff:  30 * time.Minute,
	}
}

func (p LockoutPolicy) Backoff(failures int) time.Duration {
	if failures < p.Threshold {
		return 0
	}

	over := min(failures-p.Threshold, 16)
	backoff := p.BaseBackoff * time.Duration(1<<over)
	if backoff <= 0 || backoff > p.MaxBackoff {
		return p.MaxBackoff
	}

	return backoff
}
