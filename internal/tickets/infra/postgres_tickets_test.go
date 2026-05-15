//go:build integration

package infra

import (
	"context"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/testutil"
	"github.com/hayakawakaki/go-racp/internal/tickets/domain"
)

var _ domain.Repository = (*Repository)(nil)

func setupRepo(t *testing.T) *Repository {
	t.Helper()
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_tickets")

	return NewRepository(pool)
}

func openTicketFixture(accountID int, now time.Time) (domain.Ticket, domain.Message) {
	ticket := domain.Ticket{
		AccountID:      accountID,
		AuthorUsername: "alice",
		Category:       "Other",
		Subject:        "hello",
		Status:         domain.StatusOpen,
		LastActor:      domain.ActorPlayer,
		MessageCount:   1,
		LastActivity:   now,
		CreatedAt:      now,
	}
	message := domain.Message{
		AuthorID:   accountID,
		AuthorRole: domain.ActorPlayer,
		Visibility: domain.VisibilityPublic,
		Body:       "body",
		CreatedAt:  now,
	}

	return ticket, message
}

func TestRepository_CreateAndGet(t *testing.T) {
	repo := setupRepo(t)
	now := time.Now().UTC().Truncate(time.Second)
	ticket, message := openTicketFixture(100, now)

	id, err := repo.Create(context.Background(), ticket, message)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.AccountID != 100 || got.Subject != "hello" || got.MessageCount != 1 {
		t.Errorf("got = %+v", got)
	}
	if got.AuthorUsername != "alice" {
		t.Errorf("AuthorUsername = %q", got.AuthorUsername)
	}
}

func TestRepository_AppendPublicMessage_UpdatesDenormalizedFields(t *testing.T) {
	repo := setupRepo(t)
	now := time.Now().UTC().Truncate(time.Second)
	ticket, message := openTicketFixture(100, now)
	id, err := repo.Create(context.Background(), ticket, message)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	later := now.Add(time.Minute)
	reply := domain.Message{
		TicketID:   id,
		AuthorID:   9,
		AuthorRole: domain.ActorStaff,
		Visibility: domain.VisibilityPublic,
		Body:       "staff reply",
		CreatedAt:  later,
	}
	updated, err := repo.AppendPublicMessage(context.Background(), id, reply)
	if err != nil {
		t.Fatalf("AppendPublicMessage: %v", err)
	}
	if updated.LastActor != domain.ActorStaff {
		t.Errorf("LastActor = %v", updated.LastActor)
	}
	if updated.MessageCount != 2 || !updated.LastActivity.Equal(later) {
		t.Errorf("updated = %+v", updated)
	}
}

func TestRepository_CountAndMostRecentForPlayer(t *testing.T) {
	repo := setupRepo(t)
	now := time.Now().UTC().Truncate(time.Second)
	ticket, message := openTicketFixture(100, now)
	if _, err := repo.Create(context.Background(), ticket, message); err != nil {
		t.Fatalf("Create: %v", err)
	}
	later := now.Add(2 * time.Hour)
	ticket2, message2 := openTicketFixture(100, later)
	if _, err := repo.Create(context.Background(), ticket2, message2); err != nil {
		t.Fatalf("Create 2: %v", err)
	}

	count, err := repo.CountOpenForPlayer(context.Background(), 100)
	if err != nil {
		t.Fatalf("CountOpenForPlayer: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
	at, err := repo.MostRecentOpenedAt(context.Background(), 100)
	if err != nil {
		t.Fatalf("MostRecentOpenedAt: %v", err)
	}
	if !at.Equal(later) {
		t.Errorf("at = %v, want %v", at, later)
	}
}

func TestRepository_ListForStaff_FiltersByTab(t *testing.T) {
	repo := setupRepo(t)
	now := time.Now().UTC().Truncate(time.Second)
	ticket, message := openTicketFixture(100, now)
	id, err := repo.Create(context.Background(), ticket, message)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	awaiting, total, err := repo.ListForStaff(context.Background(),
		domain.TabOpenNoResponse, []string{"Other"}, domain.Page{Limit: 20, Offset: 0})
	if err != nil {
		t.Fatalf("ListForStaff awaiting: %v", err)
	}
	if total != 1 || len(awaiting) != 1 {
		t.Errorf("awaiting total=%d list=%d, want 1/1", total, len(awaiting))
	}

	reply := domain.Message{
		TicketID:   id,
		AuthorID:   9,
		AuthorRole: domain.ActorStaff,
		Visibility: domain.VisibilityPublic,
		Body:       "reply",
		CreatedAt:  now.Add(time.Minute),
	}
	if _, err := repo.AppendPublicMessage(context.Background(), id, reply); err != nil {
		t.Fatalf("AppendPublicMessage: %v", err)
	}

	active, total, err := repo.ListForStaff(context.Background(),
		domain.TabActive, []string{"Other"}, domain.Page{Limit: 20, Offset: 0})
	if err != nil {
		t.Fatalf("ListForStaff active: %v", err)
	}
	if total != 1 || len(active) != 1 {
		t.Errorf("active total=%d list=%d, want 1/1", total, len(active))
	}
}

func TestRepository_SetTerminal(t *testing.T) {
	repo := setupRepo(t)
	now := time.Now().UTC().Truncate(time.Second)
	ticket, message := openTicketFixture(100, now)
	id, err := repo.Create(context.Background(), ticket, message)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	closed, _, err := repo.SetTerminal(context.Background(), id, domain.StatusClosed, 9, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("SetTerminal: %v", err)
	}
	if closed.Status != domain.StatusClosed || closed.ClosedBy == nil || *closed.ClosedBy != 9 {
		t.Errorf("closed = %+v", closed)
	}
}

func TestRepository_AppendSystemEvent_UpdatesCategory(t *testing.T) {
	repo := setupRepo(t)
	now := time.Now().UTC().Truncate(time.Second)
	ticket, message := openTicketFixture(100, now)
	id, err := repo.Create(context.Background(), ticket, message)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	stored, err := repo.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	later := now.Add(time.Minute)
	updated, sysMsg, err := stored.Recategorize(9, "BugReport", later)
	if err != nil {
		t.Fatalf("Recategorize: %v", err)
	}
	if err := repo.AppendSystemEvent(context.Background(), id, updated, sysMsg); err != nil {
		t.Fatalf("AppendSystemEvent: %v", err)
	}

	reloaded, err := repo.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get reloaded: %v", err)
	}
	if reloaded.Category != "BugReport" {
		t.Errorf("Category = %q, want BugReport", reloaded.Category)
	}
}
