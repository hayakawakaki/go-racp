package moderation

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	accself "github.com/hayakawakaki/go-racp/internal/features/account/app/self"
	accdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

type fakeUserRepo struct {
	users  map[int]*accdomain.User
	getErr error
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: map[int]*accdomain.User{}}
}

func (r *fakeUserRepo) GetByID(_ context.Context, id int) (*accdomain.User, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	u, ok := r.users[id]
	if !ok {
		return nil, accdomain.ErrUserNotFound
	}

	return u, nil
}

func (r *fakeUserRepo) List(_ context.Context, _ ListQuery) (UserPage, error) {
	users := make([]accdomain.User, 0, len(r.users))
	for _, u := range r.users {
		users = append(users, *u)
	}

	return UserPage{Users: users, Total: len(users), Page: 1, PerPage: 20, TotalPages: 1}, nil
}

func (r *fakeUserRepo) UpdateBan(_ context.Context, id, state int, unbanTime uint32) error {
	u, ok := r.users[id]
	if !ok {
		return accdomain.ErrUserNotFound
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
		return accdomain.ErrUserNotFound
	}
	u.GroupID = groupID

	return nil
}

type fakeCharRepo struct {
	chars map[int][]accdomain.Character
}

func (r *fakeCharRepo) ListByAccount(_ context.Context, accountID int) ([]accdomain.Character, error) {
	return r.chars[accountID], nil
}

type fakeAuditRepo struct {
	recordErr error
	rows      []accdomain.AuditEntry
}

func (r *fakeAuditRepo) Record(_ context.Context, a accdomain.AuditEntry) error {
	if r.recordErr != nil {
		return r.recordErr
	}
	a.At = time.Now()
	r.rows = append([]accdomain.AuditEntry{a}, r.rows...)

	return nil
}

func (r *fakeAuditRepo) ListByTarget(_ context.Context, targetID, limit int) ([]accdomain.AuditEntry, error) {
	out := make([]accdomain.AuditEntry, 0, len(r.rows))
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

func newTestService(t *testing.T) (*Service, *fakeUserRepo, *fakeCharRepo, *fakeAuditRepo) {
	t.Helper()
	users := newFakeUserRepo()
	chars := &fakeCharRepo{chars: map[int][]accdomain.Character{}}
	audits := &fakeAuditRepo{}
	svc := NewService(Sources{
		Users:        users,
		Characters:   chars,
		Audits:       audits,
		AllowedRoles: map[int]string{0: "Player", 20: "Moderator"},
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	return svc, users, chars, audits
}

func TestService_Get_BundlesUserCharsAndActions(t *testing.T) {
	t.Parallel()
	svc, users, chars, audits := newTestService(t)
	users.users[7] = &accdomain.User{ID: 7, Username: "kaki"}
	chars.chars[7] = []accdomain.Character{{ID: 1, Name: "Kaki1"}}
	_ = audits.Record(context.Background(), accdomain.AuditEntry{ActorUserID: 1, TargetUserID: 7, Kind: accdomain.AuditBan})

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
	if !errors.Is(err, accdomain.ErrUserNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestService_List_ReturnsPage(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[1] = &accdomain.User{ID: 1, Username: "kaki"}
	users.users[2] = &accdomain.User{ID: 2, Username: "crazyarashi"}

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
	svc, users, _, audits := newTestService(t)
	users.users[7] = &accdomain.User{ID: 7, Username: "kaki"}

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
	if len(audits.rows) != 1 || audits.rows[0].Kind != accdomain.AuditBan {
		t.Errorf("audit not recorded: %+v", audits.rows)
	}
}

func TestService_Ban_Permanent(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[7] = &accdomain.User{ID: 7, Username: "kaki"}

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
	users.users[7] = &accdomain.User{ID: 7, Username: "kaki"}

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
	users.users[7] = &accdomain.User{ID: 7, Username: "kaki"}

	_, err := svc.Ban(context.Background(), BanCommand{
		ActorUserID:  7,
		TargetUserID: 7,
		Permanent:    true,
		Reason:       "x",
	})
	if !errors.Is(err, accdomain.ErrSelfAction) {
		t.Errorf("err = %v, want ErrSelfAction", err)
	}
}

func TestService_Ban_RejectsAdminTarget(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[7] = &accdomain.User{ID: 7, Username: "kaki", GroupID: 99}

	_, err := svc.Ban(context.Background(), BanCommand{
		ActorUserID:  1,
		TargetUserID: 7,
		Permanent:    true,
		Reason:       "x",
	})
	if !errors.Is(err, accdomain.ErrTargetIsAdmin) {
		t.Errorf("err = %v, want ErrTargetIsAdmin", err)
	}
}

func TestService_Ban_RejectsEmptyReason(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[7] = &accdomain.User{ID: 7, Username: "kaki"}

	_, err := svc.Ban(context.Background(), BanCommand{
		ActorUserID:  1,
		TargetUserID: 7,
		Permanent:    true,
		Reason:       "   ",
	})
	if !errors.Is(err, accdomain.ErrEmptyReason) {
		t.Errorf("err = %v, want ErrEmptyReason", err)
	}
}

func TestService_Unban_ClearsState(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[7] = &accdomain.User{ID: 7, Username: "kaki", State: 5}

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
	users.users[7] = &accdomain.User{ID: 7, Username: "kaki"}

	_, err := svc.Unban(context.Background(), UnbanCommand{
		ActorUserID:  1,
		TargetUserID: 7,
	})
	if !errors.Is(err, accdomain.ErrInvalidState) {
		t.Errorf("err = %v, want ErrInvalidState", err)
	}
}

func TestService_Unban_RejectsAdminTarget(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[7] = &accdomain.User{ID: 7, Username: "kaki", State: 5, GroupID: 99}

	_, err := svc.Unban(context.Background(), UnbanCommand{ActorUserID: 1, TargetUserID: 7})
	if !errors.Is(err, accdomain.ErrTargetIsAdmin) {
		t.Errorf("err = %v, want ErrTargetIsAdmin", err)
	}
}

func TestService_SetRole_AllowedRole(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[7] = &accdomain.User{ID: 7, Username: "kaki", GroupID: 0}

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
	users.users[7] = &accdomain.User{ID: 7, Username: "kaki"}

	_, err := svc.SetRole(context.Background(), SetRoleCommand{
		ActorUserID:  1,
		TargetUserID: 7,
		NewGroupID:   99,
	})
	if !errors.Is(err, accdomain.ErrInvalidRole) {
		t.Errorf("err = %v, want ErrInvalidRole", err)
	}
}

func TestService_SetRole_NoOpRejected(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[7] = &accdomain.User{ID: 7, Username: "kaki", GroupID: 0}

	_, err := svc.SetRole(context.Background(), SetRoleCommand{
		ActorUserID:  1,
		TargetUserID: 7,
		NewGroupID:   0,
	})
	if !errors.Is(err, accdomain.ErrInvalidState) {
		t.Errorf("err = %v, want ErrInvalidState", err)
	}
}

func TestService_Ban_AuditFailureDoesNotRollBack(t *testing.T) {
	t.Parallel()
	svc, users, _, audits := newTestService(t)
	users.users[7] = &accdomain.User{ID: 7, Username: "kaki"}
	audits.recordErr = errors.New("postgres down")

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

func TestService_Ban_PlayerOnlyForNonAdmin(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[1] = &accdomain.User{ID: 1, Username: "modActor", GroupID: 20}
	users.users[2] = &accdomain.User{ID: 2, Username: "modTarget", GroupID: 20}
	users.users[3] = &accdomain.User{ID: 3, Username: "playerTarget", GroupID: 0}

	_, err := svc.Ban(context.Background(), BanCommand{
		ActorUserID:  1,
		ActorIsAdmin: false,
		TargetUserID: 2,
		Reason:       "test",
		Days:         1,
	})
	if !errors.Is(err, accdomain.ErrTargetProtected) {
		t.Fatalf("mod->mod: err = %v, want ErrTargetProtected", err)
	}

	_, err = svc.Ban(context.Background(), BanCommand{
		ActorUserID:  1,
		ActorIsAdmin: false,
		TargetUserID: 3,
		Reason:       "test",
		Days:         1,
	})
	if err != nil {
		t.Fatalf("mod->player: err = %v, want nil", err)
	}
}

func TestService_BanForChargeback_BansPlayer(t *testing.T) {
	t.Parallel()
	svc, users, _, audits := newTestService(t)
	users.users[7] = &accdomain.User{ID: 7, Username: "kaki", GroupID: 0, State: accself.StateActive}

	if err := svc.BanForChargeback(context.Background(), 7, "payment chargeback"); err != nil {
		t.Fatalf("BanForChargeback: %v", err)
	}
	if users.users[7].State != accself.StatePermaBanned {
		t.Errorf("state = %d, want %d", users.users[7].State, accself.StatePermaBanned)
	}
	if len(audits.rows) != 1 {
		t.Fatalf("audit rows = %d, want 1", len(audits.rows))
	}
	if audits.rows[0].ActorUserID != 0 || audits.rows[0].Kind != accdomain.AuditBan {
		t.Errorf("audit = %+v, want actor 0 and AuditBan", audits.rows[0])
	}
}

func TestService_BanForChargeback_SkipsAdmin(t *testing.T) {
	t.Parallel()
	svc, users, _, audits := newTestService(t)
	users.users[7] = &accdomain.User{ID: 7, Username: "kaki", GroupID: 99, State: accself.StateActive}

	if err := svc.BanForChargeback(context.Background(), 7, "payment chargeback"); err != nil {
		t.Fatalf("BanForChargeback: %v", err)
	}
	if users.users[7].State != accself.StateActive {
		t.Errorf("state = %d, want unchanged %d", users.users[7].State, accself.StateActive)
	}
	if len(audits.rows) != 0 {
		t.Errorf("audit rows = %d, want 0", len(audits.rows))
	}
}

func TestService_Ban_AdminCanActOnRoleBearing(t *testing.T) {
	t.Parallel()
	svc, users, _, _ := newTestService(t)
	users.users[1] = &accdomain.User{ID: 1, Username: "adminActor", GroupID: 99}
	users.users[2] = &accdomain.User{ID: 2, Username: "modTarget", GroupID: 20}

	_, err := svc.Ban(context.Background(), BanCommand{
		ActorUserID:  1,
		ActorIsAdmin: true,
		TargetUserID: 2,
		Reason:       "test",
		Days:         1,
	})
	if err != nil {
		t.Fatalf("admin->mod: err = %v, want nil", err)
	}
}
