package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/platform/notification/domain"
)

var errBoom = errors.New("boom")

var _ domain.Repository = (*fakeRepo)(nil)

type fakeRepo struct {
	pruneCutoff    time.Time
	markReadNow    time.Time
	createErr      error
	recentErr      error
	unreadErr      error
	markReadErr    error
	markAllErr     error
	pruneErr       error
	listPageErr    error
	link           string
	created        []domain.Notification
	listPageItems  []domain.Notification
	nextID         int64
	pruned         int64
	unread         int
	recentLimit    int
	listPageTotal  int
	listPageLimit  int
	listPageOffset int
	listPageUnread bool
}

func (r *fakeRepo) Create(_ context.Context, n domain.Notification) (domain.Notification, error) {
	if r.createErr != nil {
		return domain.Notification{}, r.createErr
	}

	r.nextID++
	n.ID = r.nextID
	r.created = append(r.created, n)
	if n.ReadAt == nil {
		r.unread++
	}

	return n, nil
}

func (r *fakeRepo) RecentByAccount(_ context.Context, _, limit int) ([]domain.Notification, error) {
	r.recentLimit = limit
	if r.recentErr != nil {
		return nil, r.recentErr
	}

	return r.created, nil
}

func (r *fakeRepo) ListPage(_ context.Context, _ int, unreadOnly bool, limit, offset int) ([]domain.Notification, int, error) {
	r.listPageUnread = unreadOnly
	r.listPageLimit = limit
	r.listPageOffset = offset
	if r.listPageErr != nil {
		return nil, 0, r.listPageErr
	}

	return r.listPageItems, r.listPageTotal, nil
}

func (r *fakeRepo) UnreadCount(_ context.Context, _ int) (int, error) {
	if r.unreadErr != nil {
		return 0, r.unreadErr
	}

	return r.unread, nil
}

func (r *fakeRepo) MarkRead(_ context.Context, _ int, _ int64, now time.Time) (string, error) {
	r.markReadNow = now
	if r.markReadErr != nil {
		return "", r.markReadErr
	}
	if r.unread > 0 {
		r.unread--
	}

	return r.link, nil
}

func (r *fakeRepo) MarkAllRead(_ context.Context, _ int, _ time.Time) (int64, error) {
	if r.markAllErr != nil {
		return 0, r.markAllErr
	}
	r.unread = 0

	return 0, nil
}

func (r *fakeRepo) PruneOlderThan(_ context.Context, cutoff time.Time) (int64, error) {
	r.pruneCutoff = cutoff
	if r.pruneErr != nil {
		return 0, r.pruneErr
	}

	return r.pruned, nil
}

func newService(repo domain.Repository, opts ...Option) *Service {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return NewService(repo, NewBroadcaster(), logger, opts...)
}

func TestNewService_Defaults(t *testing.T) {
	t.Parallel()

	svc := newService(&fakeRepo{})
	if svc.recentLimit != 20 {
		t.Errorf("recentLimit = %d, want 20", svc.recentLimit)
	}
	if svc.now == nil {
		t.Error("now func is nil")
	}
}

func TestNewService_NilRepoPanics(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil repo, got none")
		}
	}()

	NewService(nil, NewBroadcaster(), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestNewService_NilDependenciesGetDefaults(t *testing.T) {
	t.Parallel()

	svc := NewService(&fakeRepo{}, nil, nil)
	if svc.broadcaster == nil {
		t.Error("broadcaster is nil after NewService(nil broadcaster)")
	}
	if svc.logger == nil {
		t.Error("logger is nil after NewService(nil logger)")
	}
}

func TestWithRecentLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   int
		want int
	}{
		{"positive sets", 5, 5},
		{"zero ignored", 0, 20},
		{"negative ignored", -3, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := newService(&fakeRepo{}, WithRecentLimit(tt.in))
			if svc.recentLimit != tt.want {
				t.Errorf("recentLimit = %d, want %d", svc.recentLimit, tt.want)
			}
		})
	}
}

func TestService_Emit_InvalidAccount(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	svc := newService(repo)

	for _, accountID := range []int{0, -1} {
		if err := svc.Emit(context.Background(), accountID, "c", "t", "b", "l"); !errors.Is(err, domain.ErrInvalidAccount) {
			t.Errorf("Emit(account=%d) err = %v, want ErrInvalidAccount", accountID, err)
		}
	}
	if len(repo.created) != 0 {
		t.Errorf("created %d notifications, want 0", len(repo.created))
	}
}

func TestService_Emit_PersistsAndPublishes(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	svc := newService(repo)
	events, unsubscribe := svc.Subscribe(7)
	defer unsubscribe()

	if err := svc.Emit(context.Background(), 7, "market.sold", "Sold", "body", "/store"); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("created = %d, want 1", len(repo.created))
	}
	if got := repo.created[0]; got.AccountID != 7 || got.Category != "market.sold" || got.Link != "/store" {
		t.Errorf("created = %+v", got)
	}

	event := recvEvent(t, events)
	if event.Unread != 1 {
		t.Errorf("event.Unread = %d, want 1", event.Unread)
	}
	if event.Notification.Title != "Sold" {
		t.Errorf("event.Notification.Title = %q, want Sold", event.Notification.Title)
	}
}

func TestService_Emit_RepoErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		repo    *fakeRepo
		wantErr error
		name    string
	}{
		{&fakeRepo{createErr: errBoom}, errBoom, "create error"},
		{&fakeRepo{unreadErr: errBoom}, errBoom, "unread count error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := newService(tt.repo)
			if err := svc.Emit(context.Background(), 7, "c", "t", "b", "l"); !errors.Is(err, tt.wantErr) {
				t.Errorf("Emit err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_Recent_UsesRecentLimit(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	svc := newService(repo, WithRecentLimit(7))

	if _, err := svc.Recent(context.Background(), 1); err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if repo.recentLimit != 7 {
		t.Errorf("limit passed to repo = %d, want 7", repo.recentLimit)
	}
}

func TestService_Recent_PropagatesError(t *testing.T) {
	t.Parallel()

	svc := newService(&fakeRepo{recentErr: errBoom})
	if _, err := svc.Recent(context.Background(), 1); !errors.Is(err, errBoom) {
		t.Errorf("Recent err = %v, want errBoom", err)
	}
}

func TestService_UnreadCount(t *testing.T) {
	t.Parallel()

	svc := newService(&fakeRepo{unread: 4})
	count, err := svc.UnreadCount(context.Background(), 7)
	if err != nil {
		t.Fatalf("UnreadCount: %v", err)
	}
	if count != 4 {
		t.Errorf("count = %d, want 4", count)
	}

	failing := newService(&fakeRepo{unreadErr: errBoom})
	if _, err := failing.UnreadCount(context.Background(), 7); !errors.Is(err, errBoom) {
		t.Errorf("UnreadCount err = %v, want errBoom", err)
	}
}

func TestService_MarkRead_ReturnsLinkAndPublishes(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{link: "/tickets/3", unread: 2}
	svc := newService(repo)
	events, unsubscribe := svc.Subscribe(7)
	defer unsubscribe()

	link, err := svc.MarkRead(context.Background(), 7, 3)
	if err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if link != "/tickets/3" {
		t.Errorf("link = %q, want /tickets/3", link)
	}

	event := recvEvent(t, events)
	if event.Unread != 1 {
		t.Errorf("event.Unread = %d, want 1", event.Unread)
	}
}

func TestService_MarkRead_UsesInjectedNow(t *testing.T) {
	t.Parallel()

	fixed := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepo{}
	svc := newService(repo, WithNow(func() time.Time { return fixed }))

	if _, err := svc.MarkRead(context.Background(), 7, 1); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if !repo.markReadNow.Equal(fixed) {
		t.Errorf("now passed to repo = %v, want %v", repo.markReadNow, fixed)
	}
}

func TestService_MarkRead_PropagatesError(t *testing.T) {
	t.Parallel()

	svc := newService(&fakeRepo{markReadErr: errBoom})
	if _, err := svc.MarkRead(context.Background(), 7, 1); !errors.Is(err, errBoom) {
		t.Errorf("MarkRead err = %v, want errBoom", err)
	}
}

func TestService_MarkAllRead_PublishesZero(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{unread: 5}
	svc := newService(repo)
	events, unsubscribe := svc.Subscribe(7)
	defer unsubscribe()

	if err := svc.MarkAllRead(context.Background(), 7); err != nil {
		t.Fatalf("MarkAllRead: %v", err)
	}

	event := recvEvent(t, events)
	if event.Unread != 0 {
		t.Errorf("event.Unread = %d, want 0", event.Unread)
	}
}

func TestService_MarkAllRead_PropagatesError(t *testing.T) {
	t.Parallel()

	svc := newService(&fakeRepo{markAllErr: errBoom})
	if err := svc.MarkAllRead(context.Background(), 7); !errors.Is(err, errBoom) {
		t.Errorf("MarkAllRead err = %v, want errBoom", err)
	}
}

func TestService_Prune_DisabledWhenRetentionNonPositive(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{pruned: 9}
	svc := newService(repo)

	removed, err := svc.Prune(context.Background())
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
	if !repo.pruneCutoff.IsZero() {
		t.Errorf("PruneOlderThan was called (cutoff %v), want not called", repo.pruneCutoff)
	}
}

func TestService_Prune_ComputesCutoffFromNow(t *testing.T) {
	t.Parallel()

	fixed := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	retention := 48 * time.Hour
	repo := &fakeRepo{pruned: 4}
	svc := newService(repo,
		WithNow(func() time.Time { return fixed }),
		WithRetention(retention),
	)

	removed, err := svc.Prune(context.Background())
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if removed != 4 {
		t.Errorf("removed = %d, want 4", removed)
	}

	want := fixed.Add(-retention)
	if !repo.pruneCutoff.Equal(want) {
		t.Errorf("cutoff = %v, want %v", repo.pruneCutoff, want)
	}
}

func TestService_Prune_PropagatesError(t *testing.T) {
	t.Parallel()

	svc := newService(&fakeRepo{pruneErr: errBoom}, WithRetention(time.Hour))
	if _, err := svc.Prune(context.Background()); !errors.Is(err, errBoom) {
		t.Errorf("Prune err = %v, want errBoom", err)
	}
}

func TestService_PublishUnread_BroadcastsCount(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{unread: 4}
	svc := newService(repo)
	events, unsubscribe := svc.Subscribe(7)
	defer unsubscribe()

	svc.PublishUnread(context.Background(), 7)

	event := recvEvent(t, events)
	if event.Unread != 4 {
		t.Errorf("event.Unread = %d, want 4", event.Unread)
	}
}

func TestService_PublishUnread_RepoErrorDoesNotPublish(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{unreadErr: errBoom}
	svc := newService(repo)
	events, unsubscribe := svc.Subscribe(7)
	defer unsubscribe()

	svc.PublishUnread(context.Background(), 7)

	select {
	case got := <-events:
		t.Errorf("published event %+v despite repo error", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestService_Inbox_ComputesOffsetAndPages(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{listPageTotal: 130, listPageItems: []domain.Notification{{ID: 1}}}
	svc := newService(repo)

	result, err := svc.Inbox(context.Background(), 7, false, 3)
	if err != nil {
		t.Fatalf("Inbox: %v", err)
	}
	if repo.listPageLimit != 50 {
		t.Errorf("limit = %d, want 50", repo.listPageLimit)
	}
	if repo.listPageOffset != 100 {
		t.Errorf("offset = %d, want 100", repo.listPageOffset)
	}
	if result.Page != 3 || result.PerPage != 50 || result.Total != 130 {
		t.Errorf("page result = %+v", result)
	}
	if result.TotalPages != 3 {
		t.Errorf("TotalPages = %d, want 3", result.TotalPages)
	}
}

func TestService_Inbox_ClampsPageAndPassesFilter(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	svc := newService(repo)

	result, err := svc.Inbox(context.Background(), 7, true, 0)
	if err != nil {
		t.Fatalf("Inbox: %v", err)
	}
	if result.Page != 1 {
		t.Errorf("Page = %d, want 1 (clamped)", result.Page)
	}
	if repo.listPageOffset != 0 {
		t.Errorf("offset = %d, want 0", repo.listPageOffset)
	}
	if !repo.listPageUnread {
		t.Errorf("unreadOnly not passed through to repo")
	}
	if result.TotalPages != 1 {
		t.Errorf("TotalPages = %d, want 1 for empty result", result.TotalPages)
	}
}

func TestService_Inbox_PropagatesError(t *testing.T) {
	t.Parallel()

	svc := newService(&fakeRepo{listPageErr: errBoom})
	if _, err := svc.Inbox(context.Background(), 7, false, 1); !errors.Is(err, errBoom) {
		t.Errorf("Inbox err = %v, want errBoom", err)
	}
}

func TestService_Inbox_ClampsPageToLastWhenOutOfRange(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{listPageTotal: 120, listPageItems: []domain.Notification{{ID: 1}}}
	svc := newService(repo)

	result, err := svc.Inbox(context.Background(), 7, false, 99)
	if err != nil {
		t.Fatalf("Inbox: %v", err)
	}
	if result.Page != 3 {
		t.Errorf("Page = %d, want 3 (clamped to last page)", result.Page)
	}
	if repo.listPageOffset != 100 {
		t.Errorf("final offset = %d, want 100 (last page)", repo.listPageOffset)
	}
	if len(result.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1 (re-fetched last page)", len(result.Items))
	}
}
