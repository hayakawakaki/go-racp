package app

import (
	"context"
	"fmt"
	"time"

	"github.com/hayakawakaki/go-racp/internal/character/domain"
)

type DefaultLocation struct {
	Map string
	X   int
	Y   int
}

type Service struct {
	chars            domain.Repository
	cooldowns        domain.Cooldowns
	now              func() time.Time
	defaultLocation  DefaultLocation
	lookCooldown     time.Duration
	locationCooldown time.Duration
}

type Option func(*Service)

func WithNow(fn func() time.Time) Option {
	return func(s *Service) {
		if fn == nil {
			s.now = time.Now
			return
		}
		s.now = fn
	}
}

func WithCooldowns(look, location time.Duration) Option {
	return func(s *Service) {
		s.lookCooldown = look
		s.locationCooldown = location
	}
}

func WithDefaultLocation(loc DefaultLocation) Option {
	return func(s *Service) { s.defaultLocation = loc }
}

func NewService(chars domain.Repository, cooldowns domain.Cooldowns, opts ...Option) *Service {
	s := &Service{
		chars:     chars,
		cooldowns: cooldowns,
		now:       time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *Service) Now() time.Time {
	return s.now()
}

func (s *Service) List(ctx context.Context, accountID int) ([]CharacterDTO, error) {
	chars, err := s.chars.ListByAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("app.Service.List: %w", err)
	}

	out := make([]CharacterDTO, 0, len(chars))
	for i := range chars {
		dto, err := s.decorate(ctx, &chars[i])
		if err != nil {
			return nil, fmt.Errorf("app.Service.List: %w", err)
		}
		out = append(out, dto)
	}

	return out, nil
}

func (s *Service) Get(ctx context.Context, accountID, charID int) (*CharacterDTO, error) {
	c, err := s.chars.GetByID(ctx, charID)
	if err != nil {
		return nil, fmt.Errorf("app.Service.Get: %w", err)
	}
	if c.AccountID != accountID {
		return nil, domain.ErrNotOwner
	}
	dto, err := s.decorate(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("app.Service.Get: %w", err)
	}

	return &dto, nil
}

func (s *Service) ResetLook(ctx context.Context, accountID, charID int) error {
	if err := s.guardMutation(ctx, accountID, charID, domain.ChangeTypeLook, s.lookCooldown); err != nil {
		return err
	}
	if err := s.chars.UpdateLook(ctx, charID, 0, 0, 0); err != nil {
		return fmt.Errorf("app.Service.ResetLook: %w", err)
	}
	if err := s.cooldowns.Record(ctx, charID, domain.ChangeTypeLook, s.now()); err != nil {
		return fmt.Errorf("app.Service.ResetLook: %w", err)
	}

	return nil
}

func (s *Service) ResetLocation(ctx context.Context, accountID, charID int) error {
	if err := s.guardMutation(ctx, accountID, charID, domain.ChangeTypeLocation, s.locationCooldown); err != nil {
		return err
	}
	if err := s.chars.UpdateLocation(ctx, charID, s.defaultLocation.Map, s.defaultLocation.X, s.defaultLocation.Y); err != nil {
		return fmt.Errorf("app.Service.ResetLocation: %w", err)
	}
	if err := s.cooldowns.Record(ctx, charID, domain.ChangeTypeLocation, s.now()); err != nil {
		return fmt.Errorf("app.Service.ResetLocation: %w", err)
	}

	return nil
}

func (s *Service) guardMutation(ctx context.Context, accountID, charID int, t domain.ChangeType, window time.Duration) error {
	c, err := s.chars.GetByID(ctx, charID)
	if err != nil {
		return fmt.Errorf("app.Service.guardMutation: %w", err)
	}
	if c.AccountID != accountID {
		return domain.ErrNotOwner
	}
	if c.Online {
		return domain.ErrCharacterOnline
	}
	last, err := s.cooldowns.MostRecent(ctx, charID, t)
	if err != nil {
		return fmt.Errorf("app.Service.guardMutation: %w", err)
	}
	if !last.IsZero() && s.now().Sub(last) < window {
		return domain.ErrCooldown
	}

	return nil
}

func (s *Service) decorate(ctx context.Context, c *domain.Character) (CharacterDTO, error) {
	lookAt, err := s.cooldowns.MostRecent(ctx, c.ID, domain.ChangeTypeLook)
	if err != nil {
		return CharacterDTO{}, fmt.Errorf("look cooldown: %w", err)
	}
	locAt, err := s.cooldowns.MostRecent(ctx, c.ID, domain.ChangeTypeLocation)
	if err != nil {
		return CharacterDTO{}, fmt.Errorf("location cooldown: %w", err)
	}

	return toDTO(c, cooldownUntil(lookAt, s.lookCooldown), cooldownUntil(locAt, s.locationCooldown)), nil
}

func cooldownUntil(last time.Time, window time.Duration) time.Time {
	if last.IsZero() {
		return time.Time{}
	}

	return last.Add(window)
}

func toDTO(c *domain.Character, lookCD, locCD time.Time) CharacterDTO {
	return CharacterDTO{
		ID:            c.ID,
		Slot:          c.Slot,
		Name:          c.Name,
		Zeny:          c.Zeny,
		Gender:        c.Gender,
		JobID:         c.JobID,
		JobName:       domain.JobName(c.JobID),
		BaseLevel:     c.BaseLevel,
		JobLevel:      c.JobLevel,
		HairStyle:     c.HairStyle,
		HairColor:     c.HairColor,
		ClothesColor:  c.ClothesColor,
		BodyID:        c.BodyID,
		CurrentMap:    c.CurrentMap,
		CurrentX:      c.CurrentX,
		CurrentY:      c.CurrentY,
		SaveMap:       c.SaveMap,
		SaveX:         c.SaveX,
		SaveY:         c.SaveY,
		CostumeTop:    c.CostumeTop,
		CostumeMid:    c.CostumeMid,
		CostumeBottom: c.CostumeBottom,
		CostumeRobe:   c.CostumeRobe,
		Online:        c.Online,
		LookCDUntil:   lookCD,
		LocCDUntil:    locCD,
	}
}
