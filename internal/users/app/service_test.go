package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/users/domain"
)

type fakeUserRepo struct {
	users  map[int]*domain.User
	getErr error
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: map[int]*domain.User{}}
}

func (r *fakeUserRepo) GetByID(_ context.Context, id int) (*domain.User, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}

	return u, nil
}

func (r *fakeUserRepo) List(_ context.Context, _ ListQuery) (UserPage, error) {
	users := make([]domain.User, 0, len(r.users))
	for _, u := range r.users {
		users = append(users, *u)
	}

	return UserPage{Users: users, Total: len(users), Page: 1, PerPage: 20, TotalPages: 1}, nil
}

func (r *fakeUserRepo) UpdateBan(_ context.Context, id, state int, unbanTime uint32) error {
	u, ok := r.users[id]
	if !ok {
		return domain.ErrNotFound
	}
	u.State = state
	if unbanTime == 0 {
		u.UnbanTime = time.Time{}
	} else {
		u.UnbanTime = time.Unix(int64(unbanTime), 0)
	}

	return nil
}

func (r *fakeUserRepo) UpdateGroup(_ context.Context, id, groupID int) error {
	u, ok := r.users[id]
	if !ok {
		return domain.ErrNotFound
	}
	u.GroupID = groupID

	return nil
}

type fakeCharRepo struct {
	chars map[int][]domain.Character
}

func (r *fakeCharRepo) ListByAccount(_ context.Context, accountID int) ([]domain.Character, error) {
	return r.chars[accountID], nil
}

type fakeActionRepo struct {
	recordErr error
	rows      []domain.Action
}

func (r *fakeActionRepo) Record(_ context.Context, a domain.Action) error {
	if r.recordErr != nil {
		return r.recordErr
	}
	a.At = time.Now()
	r.rows = append([]domain.Action{a}, r.rows...)

	return nil
}

func (r *fakeActionRepo) ListByTarget(_ context.Context, targetID, limit int) ([]domain.Action, error) {
	out := make([]domain.Action, 0, len(r.rows))
	for _, a := range r.rows {
		if a.TargetUserID == targetID {
			out = append(out, a)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}

	return out, nil
}

func newTestService(t *testing.T) (*Service, *fakeUserRepo, *fakeCharRepo, *fakeActionRepo) {
	t.Helper()
	users := newFakeUserRepo()
	chars := &fakeCharRepo{chars: map[int][]domain.Character{}}
	actions := &fakeActionRepo{}
	svc := NewService(Sources{
		Users:        users,
		Characters:   chars,
		Actions:      actions,
		AllowedRoles: map[int]string{0: "Player", 20: "Moderator"},
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	return svc, users, chars, actions
}

func TestService_Get_BundlesUserCharsAndActions(t *testing.T) {
	t.Parallel()
	svc, users, chars, actions := newTestService(t)
	users.users[7] = &domain.User{ID: 7, Username: "kaki"}
	chars.chars[7] = []domain.Character{{ID: 1, Name: "Kaki1"}}
	_ = actions.Record(context.Background(), domain.Action{ActorUserID: 1, TargetUserID: 7, Kind: domain.ActionBan})

	detail, err := svc.Get(context.Background(), 7)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if detail.User.ID != 7 || len(detail.Characters) != 1 || len(detail.Recent) != 1 {
		t.Errorf("detail = %+v", detail)
	}
}

func TestService_Get_NotFound(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newTestService(t)
	_, err := svc.Get(context.Background(), 9999)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestService_List_ReturnsPage(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[1] = &domain.User{ID: 1, Username: "kaki"}
	users.users[2] = &domain.User{ID: 2, Username: "crazyarashi"}

	page, err := svc.List(context.Background(), ListQuery{Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.Total != 2 {
		t.Errorf("Total = %d, want 2", page.Total)
	}
}

func TestService_Ban_CustomTemp(t *testing.T) {
	t.Parallel()
	svc, users, _, actions := newTestService(t)
	users.users[7] = &domain.User{ID: 7, Username: "kaki"}

	detail, err := svc.Ban(context.Background(), BanCommand{
		ActorUserID:  1,
		TargetUserID: 7,
		Days:         1,
		Reason:       "spam",
	})
	if err != nil {
		t.Fatalf("Ban: %v", err)
	}
	if detail.User.UnbanTime.IsZero() {
		t.Errorf("UnbanTime should be set for temp ban")
	}
	if len(actions.rows) != 1 || actions.rows[0].Kind != domain.ActionBan {
		t.Errorf("audit not recorded: %+v", actions.rows)
	}
}

func TestService_Ban_Permanent(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[7] = &domain.User{ID: 7, Username: "kaki"}

	_, err := svc.Ban(context.Background(), BanCommand{
		ActorUserID:  1,
		TargetUserID: 7,
		Permanent:    true,
		Reason:       "rmt",
	})
	if err != nil {
		t.Fatalf("Ban: %v", err)
	}
	if users.users[7].State != 5 {
		t.Errorf("state = %d, want 5", users.users[7].State)
	}
}

func TestService_Ban_Custom(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[7] = &domain.User{ID: 7, Username: "kaki"}

	_, err := svc.Ban(context.Background(), BanCommand{
		ActorUserID:  1,
		TargetUserID: 7,
		Days:         3,
		Reason:       "harassment",
	})
	if err != nil {
		t.Fatalf("Ban: %v", err)
	}
	if users.users[7].UnbanTime.IsZero() {
		t.Errorf("UnbanTime should be set")
	}
}

func TestService_Ban_RejectsSelf(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[7] = &domain.User{ID: 7, Username: "kaki"}

	_, err := svc.Ban(context.Background(), BanCommand{
		ActorUserID:  7,
		TargetUserID: 7,
		Permanent:    true,
		Reason:       "x",
	})
	if !errors.Is(err, domain.ErrSelfAction) {
		t.Errorf("err = %v, want ErrSelfAction", err)
	}
}

func TestService_Ban_RejectsAdminTarget(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[7] = &domain.User{ID: 7, Username: "kaki", GroupID: 99}

	_, err := svc.Ban(context.Background(), BanCommand{
		ActorUserID:  1,
		TargetUserID: 7,
		Permanent:    true,
		Reason:       "x",
	})
	if !errors.Is(err, domain.ErrTargetIsAdmin) {
		t.Errorf("err = %v, want ErrTargetIsAdmin", err)
	}
}

func TestService_Ban_RejectsEmptyReason(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[7] = &domain.User{ID: 7, Username: "kaki"}

	_, err := svc.Ban(context.Background(), BanCommand{
		ActorUserID:  1,
		TargetUserID: 7,
		Permanent:    true,
		Reason:       "   ",
	})
	if !errors.Is(err, domain.ErrEmptyReason) {
		t.Errorf("err = %v, want ErrEmptyReason", err)
	}
}

func TestService_Unban_ClearsState(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[7] = &domain.User{ID: 7, Username: "kaki", State: 5}

	detail, err := svc.Unban(context.Background(), UnbanCommand{
		ActorUserID:  1,
		TargetUserID: 7,
	})
	if err != nil {
		t.Fatalf("Unban: %v", err)
	}
	if detail.User.State != 0 || !detail.User.UnbanTime.IsZero() {
		t.Errorf("unban incomplete: %+v", detail.User)
	}
}

func TestService_Unban_NotBanned(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[7] = &domain.User{ID: 7, Username: "kaki"}

	_, err := svc.Unban(context.Background(), UnbanCommand{
		ActorUserID:  1,
		TargetUserID: 7,
	})
	if !errors.Is(err, domain.ErrInvalidState) {
		t.Errorf("err = %v, want ErrInvalidState", err)
	}
}

func TestService_Unban_RejectsAdminTarget(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[7] = &domain.User{ID: 7, Username: "kaki", State: 5, GroupID: 99}

	_, err := svc.Unban(context.Background(), UnbanCommand{ActorUserID: 1, TargetUserID: 7})
	if !errors.Is(err, domain.ErrTargetIsAdmin) {
		t.Errorf("err = %v, want ErrTargetIsAdmin", err)
	}
}

func TestService_SetRole_AllowedRole(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[7] = &domain.User{ID: 7, Username: "kaki", GroupID: 0}

	detail, err := svc.SetRole(context.Background(), SetRoleCommand{
		ActorUserID:  1,
		TargetUserID: 7,
		NewGroupID:   20,
	})
	if err != nil {
		t.Fatalf("SetRole: %v", err)
	}
	if detail.User.GroupID != 20 {
		t.Errorf("GroupID = %d, want 20", detail.User.GroupID)
	}
}

func TestService_SetRole_DisallowedRole(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[7] = &domain.User{ID: 7, Username: "kaki"}

	_, err := svc.SetRole(context.Background(), SetRoleCommand{
		ActorUserID:  1,
		TargetUserID: 7,
		NewGroupID:   99,
	})
	if !errors.Is(err, domain.ErrInvalidRole) {
		t.Errorf("err = %v, want ErrInvalidRole", err)
	}
}

func TestService_SetRole_NoOpRejected(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[7] = &domain.User{ID: 7, Username: "kaki", GroupID: 0}

	_, err := svc.SetRole(context.Background(), SetRoleCommand{
		ActorUserID:  1,
		TargetUserID: 7,
		NewGroupID:   0,
	})
	if !errors.Is(err, domain.ErrInvalidState) {
		t.Errorf("err = %v, want ErrInvalidState", err)
	}
}

func TestService_Ban_AuditFailureDoesNotRollBack(t *testing.T) {
	t.Parallel()
	svc, users, _, actions := newTestService(t)
	users.users[7] = &domain.User{ID: 7, Username: "kaki"}
	actions.recordErr = errors.New("postgres down")

	detail, err := svc.Ban(context.Background(), BanCommand{
		ActorUserID:  1,
		TargetUserID: 7,
		Permanent:    true,
		Reason:       "rmt",
	})
	if err != nil {
		t.Fatalf("Ban should still succeed: %v", err)
	}
	if detail.User.State != 5 {
		t.Errorf("ban should have applied; state = %d", detail.User.State)
	}
}
