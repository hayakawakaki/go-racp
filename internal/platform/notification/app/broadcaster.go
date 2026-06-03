package app

import (
	"sync"

	"github.com/hayakawakaki/go-racp/internal/platform/notification/domain"
)

type Event struct {
	Notification domain.Notification
	Unread       int
}

type Broadcaster struct {
	subs map[int]map[chan Event]struct{}
	mu   sync.RWMutex
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{subs: make(map[int]map[chan Event]struct{})}
}

func (b *Broadcaster) Subscribe(accountID int) (events <-chan Event, cancel func()) {
	channel := make(chan Event, 8)

	b.mu.Lock()
	channels, ok := b.subs[accountID]
	if !ok {
		channels = make(map[chan Event]struct{})
		b.subs[accountID] = channels
	}
	channels[channel] = struct{}{}
	b.mu.Unlock()

	unsubscribe := func() {
		b.mu.Lock()
		if set, ok := b.subs[accountID]; ok {
			delete(set, channel)
			if len(set) == 0 {
				delete(b.subs, accountID)
			}
		}
		b.mu.Unlock()
	}

	return channel, unsubscribe
}

func (b *Broadcaster) Publish(accountID int, event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for channel := range b.subs[accountID] {
		select {
		case channel <- event:
		default:
		}
	}
}
