package domain

import (
	"errors"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)

func openTicket() Ticket {
	return Ticket{
		ID:           7,
		AccountID:    100,
		Category:     "BugReport",
		Subject:      "old subject",
		Status:       StatusOpen,
		LastActor:    ActorStaff,
		MessageCount: 2,
		LastActivity: fixedNow.Add(-time.Hour),
		CreatedAt:    fixedNow.Add(-2 * time.Hour),
	}
}

func TestNewTicket_Valid(t *testing.T) {
	t.Parallel()

	ticket, message, err := NewTicket(42, "alice", "Other", " Hi ", "  body  ", fixedNow)
	if err != nil {
		t.Fatalf("NewTicket: %v", err)
	}
	if ticket.Subject != "Hi" || ticket.AuthorUsername != "alice" {
		t.Errorf("ticket = %+v", ticket)
	}
	if message.Body != "body" || message.AuthorRole != ActorPlayer {
		t.Errorf("message = %+v", message)
	}
}

func TestNewTicket_RejectsEmptySubject(t *testing.T) {
	t.Parallel()

	_, _, err := NewTicket(1, "u", "Other", "", "body", fixedNow)
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	if _, ok := validation.Fields["subject"]; !ok {
		t.Errorf("expected subject field error, got %+v", validation.Fields)
	}
}

func TestAppendPublic_PlayerBlockedWhenLastActorPlayer(t *testing.T) {
	t.Parallel()

	ticket := openTicket()
	ticket.LastActor = ActorPlayer
	_, _, err := ticket.AppendPublic(100, ActorPlayer, "hi", fixedNow)
	if !errors.Is(err, ErrPlayerCannotReply) {
		t.Errorf("err = %v, want ErrPlayerCannotReply", err)
	}
}

func TestAppendPublic_StaffAllowedWhenLastActorPlayer(t *testing.T) {
	t.Parallel()

	ticket := openTicket()
	ticket.LastActor = ActorPlayer
	updated, message, err := ticket.AppendPublic(7, ActorStaff, "reply", fixedNow)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if updated.LastActor != ActorStaff {
		t.Errorf("LastActor = %v", updated.LastActor)
	}
	if updated.MessageCount != 3 || message.Visibility != VisibilityPublic {
		t.Errorf("updated = %+v, msg = %+v", updated, message)
	}
}

func TestTerminalRejectsAll(t *testing.T) {
	t.Parallel()

	ticket := openTicket()
	ticket.Status = StatusClosed
	cases := map[string]func() error{
		"append_public":   func() error { _, _, err := ticket.AppendPublic(1, ActorStaff, "x", fixedNow); return err },
		"append_internal": func() error { _, err := ticket.AppendInternalNote(1, "x", fixedNow); return err },
		"recategorize":    func() error { _, _, err := ticket.Recategorize(1, "Other", fixedNow); return err },
		"edit_subject":    func() error { _, _, err := ticket.EditSubject(1, "new", fixedNow); return err },
		"resolve":         func() error { _, _, err := ticket.Resolve(1, fixedNow); return err },
		"close":           func() error { _, _, err := ticket.Close(1, fixedNow); return err },
	}
	for name, run := range cases {
		if err := run(); !errors.Is(err, ErrTicketTerminal) {
			t.Errorf("%s: err = %v, want ErrTicketTerminal", name, err)
		}
	}
}

func TestRecategorize_NoOpRejected(t *testing.T) {
	t.Parallel()

	ticket := openTicket()
	_, _, err := ticket.Recategorize(1, ticket.Category, fixedNow)
	if !errors.Is(err, ErrCategoryUnchanged) {
		t.Errorf("err = %v, want ErrCategoryUnchanged", err)
	}
}

func TestRecategorize_SystemEvent(t *testing.T) {
	t.Parallel()

	ticket := openTicket()
	updated, message, err := ticket.Recategorize(9, "Donation", fixedNow)
	if err != nil {
		t.Fatalf("Recategorize: %v", err)
	}
	if updated.Category != "Donation" {
		t.Errorf("Category = %s", updated.Category)
	}
	if message.Visibility != VisibilitySystem || message.Event == nil {
		t.Errorf("message = %+v", message)
	}
}

func TestEditSubject_Unchanged(t *testing.T) {
	t.Parallel()

	ticket := openTicket()
	_, _, err := ticket.EditSubject(1, "  old subject  ", fixedNow)
	if !errors.Is(err, ErrSubjectUnchanged) {
		t.Errorf("err = %v, want ErrSubjectUnchanged", err)
	}
}

func TestResolveAndClose(t *testing.T) {
	t.Parallel()

	ticket := openTicket()
	resolved, _, err := ticket.Resolve(11, fixedNow)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Status != StatusResolved || resolved.ClosedBy == nil || *resolved.ClosedBy != 11 {
		t.Errorf("resolved = %+v", resolved)
	}

	closed, _, err := ticket.Close(12, fixedNow)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if closed.Status != StatusClosed || closed.ClosedBy == nil || *closed.ClosedBy != 12 {
		t.Errorf("closed = %+v", closed)
	}
}
