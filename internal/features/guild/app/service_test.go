package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/guild/domain"
)

type fakeRepo struct {
	guilds      map[int]*domain.Guild
	members     map[int][]domain.Member
	emblems     map[int]emblemEntry
	listHook    func(context.Context, domain.ListQuery) (domain.GuildPage, error)
	getByIDHook func(context.Context, int) (*domain.Guild, error)
	listMemHook func(context.Context, int) ([]domain.Member, error)
	getEmblHook func(context.Context, int) ([]byte, string, error)
	mu          sync.Mutex
}

type emblemEntry struct {
	err  error
	mime string
	data []byte
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		guilds:  map[int]*domain.Guild{},
		members: map[int][]domain.Member{},
		emblems: map[int]emblemEntry{},
	}
}

func (f *fakeRepo) putGuild(g domain.Guild) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := g
	f.guilds[g.ID] = &cp
}

func (f *fakeRepo) putMembers(guildID int, members []domain.Member) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.members[guildID] = members
}

func (f *fakeRepo) putEmblem(guildID int, mime string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.emblems[guildID] = emblemEntry{mime: mime, data: data}
}

func (f *fakeRepo) List(ctx context.Context, q domain.ListQuery) (domain.GuildPage, error) {
	if f.listHook != nil {
		return f.listHook(ctx, q)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]domain.Guild, 0, len(f.guilds))
	for _, g := range f.guilds {
		out = append(out, *g)
	}

	return domain.GuildPage{Guilds: out, Total: len(out), Page: q.Page, PerPage: q.PerPage}, nil
}

func (f *fakeRepo) GetByID(ctx context.Context, id int) (*domain.Guild, error) {
	if f.getByIDHook != nil {
		return f.getByIDHook(ctx, id)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	g, ok := f.guilds[id]
	if !ok {
		return nil, domain.ErrGuildNotFound
	}
	cp := *g

	return &cp, nil
}

func (f *fakeRepo) ListMembers(ctx context.Context, guildID int) ([]domain.Member, error) {
	if f.listMemHook != nil {
		return f.listMemHook(ctx, guildID)
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]domain.Member(nil), f.members[guildID]...), nil
}

func (f *fakeRepo) GetEmblem(ctx context.Context, guildID int) (data []byte, mime string, err error) {
	if f.getEmblHook != nil {
		return f.getEmblHook(ctx, guildID)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.emblems[guildID]
	if !ok {
		return nil, "", domain.ErrGuildNotFound
	}
	if e.err != nil {
		return nil, "", e.err
	}

	return append([]byte(nil), e.data...), e.mime, nil
}

func TestService_List(t *testing.T) {
	t.Parallel()

	t.Run("returns page from repo", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		repo.putGuild(domain.Guild{ID: 1, Name: "kaki", MasterName: "kaki", GuildLevel: 5, MaxMember: 16})
		svc := NewService(repo)

		page, err := svc.List(context.Background(), domain.ListQuery{Page: 1, PerPage: 20})
		if err != nil {
			t.Fatalf("List err = %v", err)
		}
		if page.Total != 1 || len(page.Guilds) != 1 {
			t.Errorf("page total/len = %d/%d, want 1/1", page.Total, len(page.Guilds))
		}
		if page.Guilds[0].Name != "kaki" {
			t.Errorf("Name = %q, want kaki", page.Guilds[0].Name)
		}
	})

	t.Run("repo error wraps", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		repo.listHook = func(context.Context, domain.ListQuery) (domain.GuildPage, error) {
			return domain.GuildPage{}, errors.New("boom")
		}
		svc := NewService(repo)

		_, err := svc.List(context.Background(), domain.ListQuery{})
		if err == nil || !strings.Contains(err.Error(), "app.Service.List") {
			t.Errorf("not wrapped: %v", err)
		}
	})
}

func TestService_Get(t *testing.T) {
	t.Parallel()

	t.Run("composes guild and members", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		repo.putGuild(domain.Guild{ID: 42, Name: "kaki", MasterName: "kaki", MasterCharID: 150000, GuildLevel: 50, MaxMember: 56})
		repo.putMembers(42, []domain.Member{
			{Name: "kaki", PositionName: "Master", CharID: 150000, Position: 0},
			{Name: "crazyarashi", PositionName: "Officer", CharID: 150001, Position: 1},
		})
		svc := NewService(repo)

		got, err := svc.Get(context.Background(), 42)
		if err != nil {
			t.Fatalf("Get err = %v", err)
		}
		if got.Guild == nil || got.Guild.ID != 42 {
			t.Fatalf("Guild = %+v, want id 42", got.Guild)
		}
		if len(got.Members) != 2 {
			t.Fatalf("Members len = %d, want 2", len(got.Members))
		}
		if got.Members[0].Name != "kaki" || got.Members[1].Name != "crazyarashi" {
			t.Errorf("members order or names wrong: %+v", got.Members)
		}
	})

	t.Run("ErrGuildNotFound propagates wrapped", func(t *testing.T) {
		t.Parallel()
		svc := NewService(newFakeRepo())

		_, err := svc.Get(context.Background(), 1)
		if !errors.Is(err, domain.ErrGuildNotFound) {
			t.Errorf("errors.Is(err, ErrGuildNotFound) = false, err = %v", err)
		}
		if err == nil || !strings.Contains(err.Error(), "app.Service.Get") {
			t.Errorf("not wrapped: %v", err)
		}
	})

	t.Run("ListMembers error wraps", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		repo.putGuild(domain.Guild{ID: 1, Name: "kaki"})
		repo.listMemHook = func(context.Context, int) ([]domain.Member, error) {
			return nil, errors.New("boom")
		}
		svc := NewService(repo)

		_, err := svc.Get(context.Background(), 1)
		if err == nil || !strings.Contains(err.Error(), "app.Service.Get") {
			t.Errorf("not wrapped: %v", err)
		}
	})

	t.Run("empty members slice is valid", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		repo.putGuild(domain.Guild{ID: 1, Name: "empty"})
		svc := NewService(repo)

		got, err := svc.Get(context.Background(), 1)
		if err != nil {
			t.Fatalf("Get err = %v", err)
		}
		if len(got.Members) != 0 {
			t.Errorf("Members len = %d, want 0", len(got.Members))
		}
	})
}

func TestService_GetEmblem(t *testing.T) {
	t.Parallel()

	t.Run("bmp passes through", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		repo.putEmblem(1, "image/bmp", []byte{'B', 'M', 0x01, 0x02})
		svc := NewService(repo)

		data, mime, err := svc.GetEmblem(context.Background(), 1)
		if err != nil {
			t.Fatalf("GetEmblem err = %v", err)
		}
		if mime != "image/bmp" {
			t.Errorf("mime = %q, want image/bmp", mime)
		}
		if string(data) != "BM\x01\x02" {
			t.Errorf("data = %x, want BM0102", data)
		}
	})

	t.Run("gif passes through", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		repo.putEmblem(1, "image/gif", []byte("GIF89a\x00\x00"))
		svc := NewService(repo)

		data, mime, err := svc.GetEmblem(context.Background(), 1)
		if err != nil {
			t.Fatalf("GetEmblem err = %v", err)
		}
		if mime != "image/gif" {
			t.Errorf("mime = %q, want image/gif", mime)
		}
		if !strings.HasPrefix(string(data), "GIF89a") {
			t.Errorf("data prefix wrong: %x", data)
		}
	})

	t.Run("propagates emblem sentinels", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			sentinel error
			name     string
		}{
			{name: "guild not found", sentinel: domain.ErrGuildNotFound},
			{name: "emblem empty", sentinel: domain.ErrEmblemEmpty},
			{name: "unknown format", sentinel: domain.ErrEmblemUnknownFormat},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				repo := newFakeRepo()
				sentinel := tc.sentinel
				repo.getEmblHook = func(context.Context, int) ([]byte, string, error) {
					return nil, "", sentinel
				}
				svc := NewService(repo)

				_, _, err := svc.GetEmblem(context.Background(), 1)
				if !errors.Is(err, sentinel) {
					t.Errorf("errors.Is = false, want %v, got %v", sentinel, err)
				}
				if err == nil || !strings.Contains(err.Error(), "app.Service.GetEmblem") {
					t.Errorf("not wrapped: %v", err)
				}
			})
		}
	})

	t.Run("unknown error wraps", func(t *testing.T) {
		t.Parallel()
		repo := newFakeRepo()
		repo.getEmblHook = func(context.Context, int) ([]byte, string, error) {
			return nil, "", errors.New("boom")
		}
		svc := NewService(repo)

		_, _, err := svc.GetEmblem(context.Background(), 1)
		if err == nil || !strings.Contains(err.Error(), "app.Service.GetEmblem") {
			t.Errorf("not wrapped: %v", err)
		}
	})
}
