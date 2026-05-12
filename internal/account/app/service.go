package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hayakawakaki/go-racp/internal/accountchange"
	"github.com/hayakawakaki/go-racp/internal/actiontoken"
	authdomain "github.com/hayakawakaki/go-racp/internal/auth/domain"
	mailtemplate "github.com/hayakawakaki/go-racp/internal/infra/mailer/template"
)

type Mailer interface {
	SendAsync(to, subject, body string)
}

type SessionInvalidator interface {
	InvalidateAllForUserExceptCurrent(ctx context.Context, userID int, currentRawToken string) error
}

type Config struct {
	AppURL                 string
	ServerName             string
	EmailChangeTokenTTL    time.Duration
	EmailChangeRequestCool time.Duration
	EmailChangeCool        time.Duration
	PasswordChangeCool     time.Duration
}

type Service struct {
	Repo           authdomain.Repository
	SessionService SessionInvalidator
	TokenManager   *actiontoken.Manager
	ChangeLog      accountchange.Repository
	Mail           Mailer
	EmailUniqueMu  *sync.Mutex
	now            func() time.Time
	cfg            Config
}

func NewService(repo authdomain.Repository, sessSvc SessionInvalidator, tokenManager *actiontoken.Manager, changeLog accountchange.Repository, mail Mailer, emailUniqueMu *sync.Mutex, cfg Config) *Service {
	if cfg.EmailChangeTokenTTL <= 0 {
		panic("account: Config.EmailChangeTokenTTL must be > 0")
	}
	if cfg.EmailChangeRequestCool <= 0 {
		panic("account: Config.EmailChangeRequestCool must be > 0")
	}
	if cfg.EmailChangeCool <= 0 {
		panic("account: Config.EmailChangeCool must be > 0")
	}
	if cfg.PasswordChangeCool <= 0 {
		panic("account: Config.PasswordChangeCool must be > 0")
	}
	return &Service{
		Repo:           repo,
		SessionService: sessSvc,
		TokenManager:   tokenManager,
		ChangeLog:      changeLog,
		Mail:           mail,
		EmailUniqueMu:  emailUniqueMu,
		now:            time.Now,
		cfg:            cfg,
	}
}

func (s *Service) GetAccount(ctx context.Context, userID int) (*AccountDTO, error) {
	user, err := s.Repo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("app.Service.GetAccount: %w", err)
	}
	return &AccountDTO{
		Username: user.Username,
		Email:    user.Email,
		Verified: user.GroupID != 5,
	}, nil
}

//nolint:cyclop // sequential validation, splitting would break the flow
func (s *Service) UpdatePassword(ctx context.Context, userID int, currentRawToken, currentPassword, newPassword, confirmPassword string) error {
	if err := authdomain.ValidatePassword(newPassword); err != nil {
		return &authdomain.ValidationError{Fields: authdomain.FieldErrors{"new_password": err.Error()}}
	}
	if err := authdomain.CheckRegistrationPassword(newPassword); err != nil {
		return &authdomain.ValidationError{Fields: authdomain.FieldErrors{"new_password": err.Error()}}
	}
	if newPassword != confirmPassword {
		return &authdomain.ValidationError{Fields: authdomain.FieldErrors{"new_password_confirm": "passwords do not match"}}
	}
	user, err := s.Repo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("app.Service.UpdatePassword: %w", err)
	}
	if user.Password != currentPassword {
		return &authdomain.ValidationError{Fields: authdomain.FieldErrors{"current_password": "incorrect"}}
	}
	lastChange, err := s.ChangeLog.MostRecent(ctx, userID, accountchange.Password)
	if err != nil {
		return fmt.Errorf("app.Service.UpdatePassword: %w", err)
	}
	if !lastChange.IsZero() && s.now().Sub(lastChange) < s.cfg.PasswordChangeCool {
		return authdomain.ErrPasswordRecentlyChanged
	}
	if err := s.Repo.UpdatePassword(ctx, userID, newPassword); err != nil {
		return fmt.Errorf("app.Service.UpdatePassword: %w", err)
	}
	if err := s.ChangeLog.Record(ctx, userID, accountchange.Password, s.now()); err != nil {
		return fmt.Errorf("app.Service.UpdatePassword: %w", err)
	}
	if err := s.SessionService.InvalidateAllForUserExceptCurrent(ctx, userID, currentRawToken); err != nil {
		return fmt.Errorf("app.Service.UpdatePassword: %w", err)
	}
	return nil
}

//nolint:cyclop // sequential validation, splitting would break the flow
func (s *Service) RequestEmailChange(ctx context.Context, userID int, currentPassword, newEmail string) error {
	normalizedNewEmail, err := authdomain.ValidateEmail(newEmail)
	if err != nil {
		return &authdomain.ValidationError{Fields: authdomain.FieldErrors{"new_email": err.Error()}}
	}
	user, err := s.Repo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("app.Service.RequestEmailChange: %w", err)
	}
	if user.Password != currentPassword {
		return &authdomain.ValidationError{Fields: authdomain.FieldErrors{"current_password": "incorrect"}}
	}
	if strings.EqualFold(normalizedNewEmail, user.Email) {
		return &authdomain.ValidationError{Fields: authdomain.FieldErrors{"new_email": "same as current"}}
	}
	existing, err := s.Repo.GetByEmail(ctx, normalizedNewEmail)
	if err != nil && !errors.Is(err, authdomain.ErrUserNotFound) {
		return fmt.Errorf("app.Service.RequestEmailChange: %w", err)
	}
	if existing != nil && existing.ID != userID {
		return &authdomain.ValidationError{Fields: authdomain.FieldErrors{"new_email": "already in use"}}
	}
	lastChange, err := s.ChangeLog.MostRecent(ctx, userID, accountchange.Email)
	if err != nil {
		return fmt.Errorf("app.Service.RequestEmailChange: %w", err)
	}
	if !lastChange.IsZero() && s.now().Sub(lastChange) < s.cfg.EmailChangeCool {
		return authdomain.ErrEmailRecentlyChanged
	}
	last, err := s.TokenManager.MostRecentIssuedAt(ctx, userID, actiontoken.EmailChange)
	if err != nil {
		return fmt.Errorf("app.Service.RequestEmailChange: %w", err)
	}
	if !last.IsZero() && s.now().Sub(last) < s.cfg.EmailChangeRequestCool {
		return ErrEmailChangeCooldown
	}
	raw, err := s.TokenManager.Issue(ctx, actiontoken.EmailChange, userID, []byte(normalizedNewEmail), s.cfg.EmailChangeTokenTTL)
	if err != nil {
		return fmt.Errorf("app.Service.RequestEmailChange: %w", err)
	}
	url := strings.TrimRight(s.cfg.AppURL, "/") + "/verify-email-change?token=" + raw
	body, err := renderEmailChangeEmail(ctx, mailtemplate.EmailChangeData{
		ServerName: s.cfg.ServerName,
		Username:   user.Username,
		NewEmail:   normalizedNewEmail,
		URL:        url,
	})
	if err != nil {
		return fmt.Errorf("app.Service.RequestEmailChange: %w", err)
	}
	subject := s.cfg.ServerName + " - confirm your new email"
	s.Mail.SendAsync(normalizedNewEmail, subject, body)
	return nil
}

func (s *Service) ConsumeEmailChange(ctx context.Context, rawToken string) (*authdomain.User, error) {
	token, err := s.TokenManager.Consume(ctx, actiontoken.EmailChange, rawToken)
	if err != nil {
		return nil, fmt.Errorf("app.Service.ConsumeEmailChange: %w", err)
	}
	newEmail := string(token.Payload)
	if _, validateErr := authdomain.ValidateEmail(newEmail); validateErr != nil {
		return nil, actiontoken.ErrTokenInvalid
	}

	s.EmailUniqueMu.Lock()
	defer s.EmailUniqueMu.Unlock()

	existing, err := s.Repo.GetByEmail(ctx, newEmail)
	if err != nil && !errors.Is(err, authdomain.ErrUserNotFound) {
		return nil, fmt.Errorf("app.Service.ConsumeEmailChange: %w", err)
	}
	if existing != nil && existing.ID != token.AccountID {
		return nil, authdomain.ErrEmailTaken
	}
	if updateErr := s.Repo.UpdateEmail(ctx, token.AccountID, newEmail); updateErr != nil {
		return nil, fmt.Errorf("app.Service.ConsumeEmailChange: %w", updateErr)
	}
	if recordErr := s.ChangeLog.Record(ctx, token.AccountID, accountchange.Email, s.now()); recordErr != nil {
		return nil, fmt.Errorf("app.Service.ConsumeEmailChange: %w", recordErr)
	}
	user, err := s.Repo.GetByID(ctx, token.AccountID)
	if err != nil {
		return nil, fmt.Errorf("app.Service.ConsumeEmailChange: %w", err)
	}
	return user, nil
}
