package app

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"strconv"
	"strings"
	"time"

	accountapp "github.com/hayakawakaki/go-racp/internal/account/app"
	"github.com/hayakawakaki/go-racp/internal/users/domain"
)

type UserRepo interface {
	GetByID(ctx context.Context, id int) (*domain.User, error)
	List(ctx context.Context, q ListQuery) (UserPage, error)
	UpdateBan(ctx context.Context, id, state int, unbanTime uint32) error
	UpdateGroup(ctx context.Context, id, groupID int) error
}

type CharRepo interface {
	ListByAccount(ctx context.Context, accountID int) ([]domain.Character, error)
}

type ActionRepo interface {
	Record(ctx context.Context, a domain.Action) error
	ListByTarget(ctx context.Context, targetID, limit int) ([]domain.Action, error)
}

type Sources struct {
	Users        UserRepo
	Characters   CharRepo
	Actions      ActionRepo
	AllowedRoles map[int]string
	Logger       *slog.Logger
}

type Service struct {
	users        UserRepo
	characters   CharRepo
	actions      ActionRepo
	allowedRoles map[int]string
	logger       *slog.Logger
}

func NewService(in Sources) *Service {
	logger := in.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		users:        in.Users,
		characters:   in.Characters,
		actions:      in.Actions,
		allowedRoles: maps.Clone(in.AllowedRoles),
		logger:       logger,
	}
}

func (s *Service) AllowedRoles() map[int]string {
	return maps.Clone(s.allowedRoles)
}

func (s *Service) List(ctx context.Context, q ListQuery) (UserPage, error) {
	page, err := s.users.List(ctx, q)
	if err != nil {
		return UserPage{}, fmt.Errorf("app.Service.List: %w", err)
	}

	return page, nil
}

func (s *Service) Get(ctx context.Context, id int) (UserDetail, error) {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return UserDetail{}, fmt.Errorf("app.Service.Get: %w", err)
	}
	chars, err := s.characters.ListByAccount(ctx, id)
	if err != nil {
		return UserDetail{}, fmt.Errorf("app.Service.Get chars: %w", err)
	}
	recent, err := s.actions.ListByTarget(ctx, id, 10)
	if err != nil {
		return UserDetail{}, fmt.Errorf("app.Service.Get actions: %w", err)
	}

	return UserDetail{User: user, Characters: chars, Recent: recent}, nil
}

func (s *Service) loadMutableTarget(ctx context.Context, actorID, targetID int) (*domain.User, error) {
	if actorID == targetID {
		return nil, domain.ErrSelfAction
	}
	target, err := s.users.GetByID(ctx, targetID)
	if err != nil {
		return nil, fmt.Errorf("loadMutableTarget: %w", err)
	}
	if target.IsAdmin() {
		return nil, domain.ErrTargetIsAdmin
	}

	return target, nil
}

func (s *Service) Ban(ctx context.Context, cmd BanCommand) (UserDetail, error) {
	reason := strings.TrimSpace(cmd.Reason)
	if reason == "" {
		return UserDetail{}, fmt.Errorf("app.Service.Ban: %w", domain.ErrEmptyReason)
	}

	target, err := s.loadMutableTarget(ctx, cmd.ActorUserID, cmd.TargetUserID)
	if err != nil {
		return UserDetail{}, fmt.Errorf("app.Service.Ban: %w", err)
	}

	var dur domain.BanDuration
	if cmd.Permanent {
		dur = domain.BanDuration{Permanent: true}
	} else {
		dur, err = domain.ParseBanDays(cmd.Days)
		if err != nil {
			return UserDetail{}, fmt.Errorf("app.Service.Ban duration: %w", err)
		}
	}

	beforeState := target.State
	beforeUnban := unbanSeconds(target.UnbanTime)
	var newState int
	var newUnban uint32
	if dur.Permanent {
		newState = accountapp.StatePermaBanned
		newUnban = 0
	} else {
		newState = accountapp.StateActive
		newUnban = unbanSeconds(time.Now().Add(dur.Duration))
	}

	if err := s.users.UpdateBan(ctx, cmd.TargetUserID, newState, newUnban); err != nil {
		return UserDetail{}, fmt.Errorf("app.Service.Ban update: %w", err)
	}

	s.recordAudit(ctx, domain.Action{
		ActorUserID:  cmd.ActorUserID,
		TargetUserID: cmd.TargetUserID,
		Kind:         domain.ActionBan,
		Reason:       reason,
		BeforeValue:  fmt.Sprintf("%d,%d", beforeState, beforeUnban),
		AfterValue:   fmt.Sprintf("%d,%d", newState, newUnban),
	})

	return s.Get(ctx, cmd.TargetUserID)
}

func (s *Service) Unban(ctx context.Context, cmd UnbanCommand) (UserDetail, error) {
	target, err := s.loadMutableTarget(ctx, cmd.ActorUserID, cmd.TargetUserID)
	if err != nil {
		return UserDetail{}, fmt.Errorf("app.Service.Unban: %w", err)
	}

	banned := target.State == accountapp.StatePermaBanned ||
		(target.State == accountapp.StateActive && !target.UnbanTime.IsZero() && target.UnbanTime.After(time.Now()))
	if !banned {
		return UserDetail{}, fmt.Errorf("app.Service.Unban: %w", domain.ErrInvalidState)
	}

	beforeState := target.State
	beforeUnban := unbanSeconds(target.UnbanTime)

	if err := s.users.UpdateBan(ctx, cmd.TargetUserID, accountapp.StateActive, 0); err != nil {
		return UserDetail{}, fmt.Errorf("app.Service.Unban update: %w", err)
	}

	s.recordAudit(ctx, domain.Action{
		ActorUserID:  cmd.ActorUserID,
		TargetUserID: cmd.TargetUserID,
		Kind:         domain.ActionUnban,
		Reason:       strings.TrimSpace(cmd.Reason),
		BeforeValue:  fmt.Sprintf("%d,%d", beforeState, beforeUnban),
		AfterValue:   "0,0",
	})

	return s.Get(ctx, cmd.TargetUserID)
}

func (s *Service) SetRole(ctx context.Context, cmd SetRoleCommand) (UserDetail, error) {
	if _, ok := s.allowedRoles[cmd.NewGroupID]; !ok {
		return UserDetail{}, fmt.Errorf("app.Service.SetRole: %w", domain.ErrInvalidRole)
	}

	target, err := s.loadMutableTarget(ctx, cmd.ActorUserID, cmd.TargetUserID)
	if err != nil {
		return UserDetail{}, fmt.Errorf("app.Service.SetRole: %w", err)
	}
	if target.GroupID == cmd.NewGroupID {
		return UserDetail{}, fmt.Errorf("app.Service.SetRole: %w", domain.ErrInvalidState)
	}

	before := target.GroupID
	if err := s.users.UpdateGroup(ctx, cmd.TargetUserID, cmd.NewGroupID); err != nil {
		return UserDetail{}, fmt.Errorf("app.Service.SetRole update: %w", err)
	}

	s.recordAudit(ctx, domain.Action{
		ActorUserID:  cmd.ActorUserID,
		TargetUserID: cmd.TargetUserID,
		Kind:         domain.ActionSetRole,
		Reason:       strings.TrimSpace(cmd.Reason),
		BeforeValue:  strconv.Itoa(before),
		AfterValue:   strconv.Itoa(cmd.NewGroupID),
	})

	return s.Get(ctx, cmd.TargetUserID)
}

func (s *Service) recordAudit(ctx context.Context, a domain.Action) {
	if err := s.actions.Record(ctx, a); err != nil {
		s.logger.Error("users: audit insert failed",
			"action", string(a.Kind),
			"actor_user_id", a.ActorUserID,
			"target_user_id", a.TargetUserID,
			"reason", a.Reason,
			"before", a.BeforeValue,
			"after", a.AfterValue,
			"err", err,
		)
	}
}

func unbanSeconds(t time.Time) uint32 {
	if t.IsZero() {
		return 0
	}

	return uint32(t.Unix()) //nolint:gosec // rAthena unban_time is uint32
}
