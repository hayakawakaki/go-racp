package app

import (
	"testing"
	"time"
)

func recvEvent(t *testing.T, ch <-chan Event) Event {
	t.Helper()

	select {
	case event := <-ch:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")

		return Event{}
	}
}

func TestBroadcaster_SubscribeReceivesPublish(t *testing.T) {
	t.Parallel()

	b := NewBroadcaster()
	events, unsubscribe := b.Subscribe(7)
	defer unsubscribe()

	b.Publish(7, Event{Unread: 3})

	if got := recvEvent(t, events); got.Unread != 3 {
		t.Errorf("Unread = %d, want 3", got.Unread)
	}
}

func TestBroadcaster_PublishToOtherAccountNotDelivered(t *testing.T) {
	t.Parallel()

	b := NewBroadcaster()
	events, unsubscribe := b.Subscribe(7)
	defer unsubscribe()

	b.Publish(8, Event{Unread: 1})

	select {
	case got := <-events:
		t.Errorf("received unexpected event %+v meant for another account", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestBroadcaster_PublishNoSubscribers(t *testing.T) {
	t.Parallel()

	b := NewBroadcaster()
	b.Publish(7, Event{Unread: 1})
}

func TestBroadcaster_MultipleSubscribersAllReceive(t *testing.T) {
	t.Parallel()

	b := NewBroadcaster()
	first, cancelFirst := b.Subscribe(7)
	defer cancelFirst()
	second, cancelSecond := b.Subscribe(7)
	defer cancelSecond()

	b.Publish(7, Event{Unread: 5})

	if got := recvEvent(t, first); got.Unread != 5 {
		t.Errorf("first Unread = %d, want 5", got.Unread)
	}
	if got := recvEvent(t, second); got.Unread != 5 {
		t.Errorf("second Unread = %d, want 5", got.Unread)
	}
}

func TestBroadcaster_UnsubscribeStopsDeliveryAndCleansUp(t *testing.T) {
	t.Parallel()

	b := NewBroadcaster()
	events, unsubscribe := b.Subscribe(7)

	unsubscribe()

	b.Publish(7, Event{Unread: 1})

	select {
	case got := <-events:
		t.Errorf("received event %+v after unsubscribe", got)
	case <-time.After(50 * time.Millisecond):
	}

	b.mu.RLock()
	_, exists := b.subs[7]
	b.mu.RUnlock()
	if exists {
		t.Errorf("account 7 still tracked after its last subscriber unsubscribed")
	}
}

func TestBroadcaster_PublishNonBlockingWhenBufferFull(t *testing.T) {
	t.Parallel()

	b := NewBroadcaster()
	_, unsubscribe := b.Subscribe(7)
	defer unsubscribe()

	done := make(chan struct{})
	go func() {
		for i := range 100 {
			b.Publish(7, Event{Unread: i})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish blocked while a subscriber buffer was full")
	}
}
