package testutil

import (
	"sync"
	"time"
)

type Clock struct {
	t  time.Time
	mu sync.Mutex
}

func NewClock(start time.Time) *Clock { return &Clock{t: start} }

func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}
