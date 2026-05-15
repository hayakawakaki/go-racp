package domain

import (
	"testing"
	"time"
)

func TestTicket_IsTerminal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Ticket
		want bool
	}{
		{"open", Ticket{Status: StatusOpen}, false},
		{"resolved", Ticket{Status: StatusResolved}, true},
		{"closed", Ticket{Status: StatusClosed}, true},
	}
	for _, tt := range tests {
		if got := tt.in.IsTerminal(); got != tt.want {
			t.Errorf("%s: IsTerminal() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestTicket_CanPlayerReply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Ticket
		want bool
	}{
		{"open + staff last", Ticket{Status: StatusOpen, LastActor: ActorStaff}, true},
		{"open + player last", Ticket{Status: StatusOpen, LastActor: ActorPlayer}, false},
		{"resolved + staff last", Ticket{Status: StatusResolved, LastActor: ActorStaff}, false},
		{"closed + staff last", Ticket{Status: StatusClosed, LastActor: ActorStaff}, false},
	}
	for _, tt := range tests {
		if got := tt.in.CanPlayerReply(); got != tt.want {
			t.Errorf("%s: CanPlayerReply() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestTicket_IsUnreadFor(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		activity   time.Time
		lastViewed time.Time
		name       string
		want       bool
	}{
		{base.Add(time.Hour), base, "activity after view", true},
		{base, base, "activity equal view", false},
		{base, base.Add(time.Hour), "activity before view", false},
	}
	for _, tt := range tests {
		ticket := Ticket{LastActivity: tt.activity}
		if got := ticket.IsUnreadFor(tt.lastViewed); got != tt.want {
			t.Errorf("%s: IsUnreadFor() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
