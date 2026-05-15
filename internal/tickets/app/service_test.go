package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	accountdomain "github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/tickets/domain"
)

type fakeRepo struct {
	tickets    map[int64]domain.Ticket
	lastOpened time.Time
	nextID     int64
	openCount  int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{tickets: map[int64]domain.Ticket{}, nextID: 1}
}

func (r *fakeRepo) Create(_ context.Context, ticket domain.Ticket, _ domain.Message) (int64, error) {
	id := r.nextID
	r.nextID++
	ticket.ID = id
	r.tickets[id] = ticket
	r.openCount++
	r.lastOpened = ticket.CreatedAt

	return id, nil
}

func (r *fakeRepo) Get(_ context.Context, id int64) (domain.Ticket, error) {
	ticket, ok := r.tickets[id]
	if !ok {
		return domain.Ticket{}, domain.ErrTicketNotFound
	}

	return ticket, nil
}

func (r *fakeRepo) ListForPlayer(context.Context, int, domain.Page) ([]domain.Ticket, int, error) {
	return nil, 0, nil
}

func (r *fakeRepo) ListForStaff(context.Context, domain.StaffTab, []string, domain.Page) ([]domain.Ticket, int, error) {
	return nil, 0, nil
}

func (r *fakeRepo) CountOpenForPlayer(context.Context, int) (int, error) { return r.openCount, nil }

func (r *fakeRepo) MostRecentOpenedAt(context.Context, int) (time.Time, error) {
	return r.lastOpened, nil
}

func (r *fakeRepo) AppendPublicMessage(_ context.Context, id int64, message domain.Message) (domain.Ticket, error) {
	ticket := r.tickets[id]
	ticket.LastActor = message.AuthorRole
	ticket.MessageCount++
	ticket.LastActivity = message.CreatedAt
	r.tickets[id] = ticket

	return ticket, nil
}

func (r *fakeRepo) AppendInternalNote(context.Context, int64, domain.Message) error { return nil }

func (r *fakeRepo) AppendSystemEvent(_ context.Context, id int64, updated domain.Ticket, _ domain.Message) error {
	r.tickets[id] = updated
	return nil
}

func (r *fakeRepo) SetTerminal(_ context.Context, id int64, status domain.Status, staffID int, at time.Time) (domain.Ticket, domain.Message, error) {
	ticket := r.tickets[id]
	ticket.Status = status
	ticket.ClosedBy = &staffID
	ticket.LastActivity = at
	r.tickets[id] = ticket

	return ticket, domain.Message{}, nil
}

type fakeMessages struct{}

func (f fakeMessages) List(context.Context, int64, bool) ([]domain.Message, error) {
	return nil, nil
}

type fakeViews struct{}

func (f fakeViews) Get(context.Context, int, int64) (time.Time, error)              { return time.Time{}, nil }
func (f fakeViews) Upsert(context.Context, int, int64, time.Time) error             { return nil }
func (f fakeViews) UnreadCountForPlayer(context.Context, int) (int, error)          { return 0, nil }
func (f fakeViews) UnreadCountForStaff(context.Context, int, []string) (int, error) { return 0, nil }
func (f fakeViews) OtherSeenAt(context.Context, int64, int, bool) (time.Time, error) {
	return time.Time{}, nil
}

type fakeUsers struct {
	user *accountdomain.User
	err  error
}

func (f fakeUsers) GetByID(context.Context, int) (*accountdomain.User, error) {
	return f.user, f.err
}

type fakeMailer struct {
	calls []string
}

func (m *fakeMailer) SendAsync(to, _, _ string) {
	m.calls = append(m.calls, to)
}

func newService(repo *fakeRepo, mailer *fakeMailer, users fakeUsers) *Service {
	resolver := domain.NewCategoryResolver([]domain.Category{
		{Key: "Other", Display: "Other", Roles: []string{"*"}},
		{Key: "BugReport", Display: "Bug", Roles: []string{"Moderator"}},
	})

	return NewService(repo, fakeMessages{}, fakeViews{}, resolver, users, mailer,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		Config{
			AppURL: "https://example.test", ServerName: "Test",
			MaxOpenPerPlayer: 2, TicketOpenCooldown: time.Minute,
		})
}

func TestOpenTicket_BlockedByMaxOpen(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	repo.openCount = 2
	service := newService(repo, &fakeMailer{},
		fakeUsers{user: &accountdomain.User{Username: "alice", Email: "a@b"}})
	_, err := service.OpenTicket(context.Background(), 1, "Other", "subj", "body")
	if !errors.Is(err, domain.ErrTooManyOpenTickets) {
		t.Errorf("err = %v, want ErrTooManyOpenTickets", err)
	}
}

func TestOpenTicket_BlockedByCooldown(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	repo.lastOpened = time.Now()
	service := newService(repo, &fakeMailer{},
		fakeUsers{user: &accountdomain.User{Username: "alice", Email: "a@b"}})
	_, err := service.OpenTicket(context.Background(), 1, "Other", "subj", "body")
	if !errors.Is(err, domain.ErrTicketCooldown) {
		t.Errorf("err = %v, want ErrTicketCooldown", err)
	}
}

func TestOpenTicket_UnknownCategory(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	service := newService(repo, &fakeMailer{},
		fakeUsers{user: &accountdomain.User{Username: "alice", Email: "a@b"}})
	_, err := service.OpenTicket(context.Background(), 1, "Nope", "subj", "body")
	if !errors.Is(err, domain.ErrUnknownCategory) {
		t.Errorf("err = %v, want ErrUnknownCategory", err)
	}
}

func TestOpenTicket_Succeeds(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	service := newService(repo, &fakeMailer{},
		fakeUsers{user: &accountdomain.User{Username: "alice", Email: "a@b"}})
	id, err := service.OpenTicket(context.Background(), 1, "Other", "subj", "body")
	if err != nil {
		t.Fatalf("OpenTicket: %v", err)
	}
	if id != 1 {
		t.Errorf("id = %d, want 1", id)
	}
	if got := repo.tickets[1].AuthorUsername; got != "alice" {
		t.Errorf("AuthorUsername = %q", got)
	}
}

func TestStaffReply_NotifiesMailer(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	repo.tickets[1] = domain.Ticket{
		ID: 1, AccountID: 100, Status: domain.StatusOpen, LastActor: domain.ActorPlayer,
	}
	repo.nextID = 2
	mailer := &fakeMailer{}
	service := newService(repo, mailer,
		fakeUsers{user: &accountdomain.User{Username: "alice", Email: "alice@example"}})

	if err := service.StaffReply(context.Background(), 9, 1, "hi"); err != nil {
		t.Fatalf("StaffReply: %v", err)
	}
	if len(mailer.calls) != 1 || mailer.calls[0] != "alice@example" {
		t.Errorf("mailer.calls = %v", mailer.calls)
	}
}

func TestStaffNote_DoesNotNotify(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	repo.tickets[1] = domain.Ticket{ID: 1, AccountID: 100, Status: domain.StatusOpen, LastActor: domain.ActorPlayer}
	mailer := &fakeMailer{}
	service := newService(repo, mailer,
		fakeUsers{user: &accountdomain.User{Username: "alice", Email: "alice@example"}})

	if err := service.StaffNote(context.Background(), 9, 1, "secret"); err != nil {
		t.Fatalf("StaffNote: %v", err)
	}
	if len(mailer.calls) != 0 {
		t.Errorf("mailer.calls = %v, want empty", mailer.calls)
	}
}

func TestPlayerReply_RejectsNonOwner(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	repo.tickets[1] = domain.Ticket{ID: 1, AccountID: 100, Status: domain.StatusOpen, LastActor: domain.ActorStaff}
	service := newService(repo, &fakeMailer{},
		fakeUsers{user: &accountdomain.User{Username: "alice", Email: "x"}})
	err := service.PlayerReply(context.Background(), 999, 1, "hi")
	if !errors.Is(err, domain.ErrNotTicketOwner) {
		t.Errorf("err = %v, want ErrNotTicketOwner", err)
	}
}

func TestStaffResolve_NotifiesAndSetsStatus(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	repo.tickets[1] = domain.Ticket{ID: 1, AccountID: 100, Status: domain.StatusOpen, LastActor: domain.ActorStaff}
	mailer := &fakeMailer{}
	service := newService(repo, mailer,
		fakeUsers{user: &accountdomain.User{Username: "alice", Email: "alice@example"}})

	if err := service.StaffResolve(context.Background(), 9, 1); err != nil {
		t.Fatalf("StaffResolve: %v", err)
	}
	if repo.tickets[1].Status != domain.StatusResolved {
		t.Errorf("status = %v", repo.tickets[1].Status)
	}
	if len(mailer.calls) != 1 {
		t.Errorf("mailer.calls = %v", mailer.calls)
	}
}
