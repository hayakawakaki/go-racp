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
	"github.com/hayakawakaki/go-racp/internal/auth/domain"
	mailtemplate "github.com/hayakawakaki/go-racp/internal/infra/mailer/template"
)

type Mailer interface {
	SendAsync(to, subject, body string)
}

type SessionInvalidator interface {
	InvalidateAllForUser(ctx context.Context, userID int) error
	InvalidateAllForUserExceptCurrent(ctx context.Context, userID int, currentRawToken string) error
}

type VerificationConfig struct {
	AppURL         string
	ServerName     string
	TokenTTL       time.Duration
	ResendCooldown time.Duration
}

type PasswordResetConfig struct {
	AppURL         string
	ServerName     string
	TokenTTL       time.Duration
	ResendCooldown time.Duration
	ChangeCooldown time.Duration
}

type Service struct {
	Repo               domain.Repository
	TokenManager       *actiontoken.Manager
	ChangeLog          accountchange.Repository
	Mail               Mailer
	SessionInvalidator SessionInvalidator
	EmailUniqueMu      *sync.Mutex
	now                func() time.Time
	verifyCfg          VerificationConfig
	resetCfg           PasswordResetConfig
	enableVerify       bool
	enableReset        bool
}

type Option func(*Service)

func WithVerification(manager *actiontoken.Manager, mail Mailer, cfg VerificationConfig) Option {
	if cfg.TokenTTL <= 0 {
		panic("auth: VerificationConfig.TokenTTL must be > 0")
	}
	if cfg.ResendCooldown <= 0 {
		panic("auth: VerificationConfig.ResendCooldown must be > 0")
	}
	return func(s *Service) {
		s.TokenManager = manager
		s.Mail = mail
		s.verifyCfg = cfg
		s.enableVerify = true
	}
}

func WithPasswordReset(manager *actiontoken.Manager, mail Mailer, cfg PasswordResetConfig) Option {
	if cfg.TokenTTL <= 0 {
		panic("auth: PasswordResetConfig.TokenTTL must be > 0")
	}
	if cfg.ResendCooldown <= 0 {
		panic("auth: PasswordResetConfig.ResendCooldown must be > 0")
	}
	if cfg.ChangeCooldown <= 0 {
		panic("auth: PasswordResetConfig.ChangeCooldown must be > 0")
	}
	return func(s *Service) {
		s.TokenManager = manager
		s.Mail = mail
		s.resetCfg = cfg
		s.enableReset = true
	}
}

func WithChangeLog(log accountchange.Repository) Option {
	return func(s *Service) { s.ChangeLog = log }
}

func WithEmailUniquenessLock(mu *sync.Mutex) Option {
	return func(s *Service) { s.EmailUniqueMu = mu }
}

func WithSessionInvalidator(invalidator SessionInvalidator) Option {
	return func(s *Service) { s.SessionInvalidator = invalidator }
}

func NewService(repo domain.Repository, opts ...Option) *Service {
	s := &Service{Repo: repo, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) Create(ctx context.Context, cmd CreateCommand) (*GetDTO, error) {
	normalizedEmail, err := validateRegistration(cmd)
	if err != nil {
		return nil, err
	}

	if s.EmailUniqueMu != nil {
		s.EmailUniqueMu.Lock()
		defer s.EmailUniqueMu.Unlock()
	}

	err = s.checkRegistrationUniqueness(ctx, cmd.Username, normalizedEmail)
	if err != nil {
		return nil, err
	}

	created, err := s.Repo.Create(ctx, &domain.User{
		Username: cmd.Username,
		Password: cmd.Password,
		Email:    normalizedEmail,
		Gender:   cmd.Gender,
	})
	if err != nil {
		return nil, fmt.Errorf("app.Service.Create: %w", err)
	}

	if s.enableVerify {
		if err := s.IssueVerification(ctx, created.ID, created.Email, created.Username); err != nil {
			return nil, fmt.Errorf("app.Service.Create: %w", err)
		}
	}

	return &GetDTO{
		ID:       created.ID,
		Username: created.Username,
		Email:    created.Email,
	}, nil
}

func (s *Service) IssueVerification(ctx context.Context, accountID int, email, username string) error {
	raw, err := s.TokenManager.Issue(ctx, actiontoken.EmailVerification, accountID, nil, s.verifyCfg.TokenTTL)
	if err != nil {
		return fmt.Errorf("app.Service.IssueVerification: %w", err)
	}
	url := strings.TrimRight(s.verifyCfg.AppURL, "/") + "/verify?token=" + raw
	body, err := renderVerificationEmail(ctx, mailtemplate.VerificationData{
		ServerName: s.verifyCfg.ServerName,
		Username:   username,
		URL:        url,
	})
	if err != nil {
		return fmt.Errorf("app.Service.IssueVerification: %w", err)
	}
	subject := s.verifyCfg.ServerName + " - verify your email"
	s.Mail.SendAsync(email, subject, body)
	return nil
}

func (s *Service) ConsumeVerification(ctx context.Context, rawToken string) error {
	token, err := s.TokenManager.Consume(ctx, actiontoken.EmailVerification, rawToken)
	if err != nil {
		return fmt.Errorf("app.Service.ConsumeVerification: %w", err)
	}
	if err := s.Repo.MarkVerified(ctx, token.AccountID); err != nil {
		return fmt.Errorf("app.Service.ConsumeVerification: %w", err)
	}
	return nil
}

func (s *Service) ResendVerification(ctx context.Context, accountID int) error {
	user, err := s.Repo.GetByID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("app.Service.ResendVerification: %w", err)
	}
	if user.GroupID != 5 {
		return nil
	}
	last, err := s.TokenManager.MostRecentIssuedAt(ctx, accountID, actiontoken.EmailVerification)
	if err != nil {
		return fmt.Errorf("app.Service.ResendVerification: %w", err)
	}
	if !last.IsZero() && s.now().Sub(last) < s.verifyCfg.ResendCooldown {
		return nil
	}
	return s.IssueVerification(ctx, accountID, user.Email, user.Username)
}

//nolint:cyclop // sequential validation + repo + change-log + token + mail steps; splitting would obscure the flow
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	normalizedEmail, err := domain.ValidateEmail(email)
	if err != nil {
		return &domain.ValidationError{Fields: domain.FieldErrors{"email": err.Error()}}
	}
	user, err := s.Repo.GetByEmail(ctx, normalizedEmail)
	if errors.Is(err, domain.ErrUserNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("app.Service.RequestPasswordReset: %w", err)
	}
	lastChange, err := s.ChangeLog.MostRecent(ctx, user.ID, accountchange.Password)
	if err != nil {
		return fmt.Errorf("app.Service.RequestPasswordReset: %w", err)
	}
	if !lastChange.IsZero() && s.now().Sub(lastChange) < s.resetCfg.ChangeCooldown {
		return nil
	}
	last, err := s.TokenManager.MostRecentIssuedAt(ctx, user.ID, actiontoken.PasswordReset)
	if err != nil {
		return fmt.Errorf("app.Service.RequestPasswordReset: %w", err)
	}
	if !last.IsZero() && s.now().Sub(last) < s.resetCfg.ResendCooldown {
		return nil
	}
	raw, err := s.TokenManager.Issue(ctx, actiontoken.PasswordReset, user.ID, nil, s.resetCfg.TokenTTL)
	if err != nil {
		return fmt.Errorf("app.Service.RequestPasswordReset: %w", err)
	}
	url := strings.TrimRight(s.resetCfg.AppURL, "/") + "/reset-password?token=" + raw
	body, err := renderPasswordResetEmail(ctx, mailtemplate.PasswordResetData{
		ServerName: s.resetCfg.ServerName,
		Username:   user.Username,
		URL:        url,
	})
	if err != nil {
		return fmt.Errorf("app.Service.RequestPasswordReset: %w", err)
	}
	subject := s.resetCfg.ServerName + " - reset your password"
	s.Mail.SendAsync(user.Email, subject, body)
	return nil
}

func (s *Service) ConsumePasswordReset(ctx context.Context, rawToken, newPassword string) error {
	if err := domain.ValidatePassword(newPassword); err != nil {
		return &domain.ValidationError{Fields: domain.FieldErrors{"password": err.Error()}}
	}
	if err := domain.CheckRegistrationPassword(newPassword); err != nil {
		return &domain.ValidationError{Fields: domain.FieldErrors{"password": err.Error()}}
	}
	token, err := s.TokenManager.Consume(ctx, actiontoken.PasswordReset, rawToken)
	if err != nil {
		return fmt.Errorf("app.Service.ConsumePasswordReset: %w", err)
	}
	if err := s.Repo.UpdatePassword(ctx, token.AccountID, newPassword); err != nil {
		return fmt.Errorf("app.Service.ConsumePasswordReset: %w", err)
	}
	if err := s.ChangeLog.Record(ctx, token.AccountID, accountchange.Password, s.now()); err != nil {
		return fmt.Errorf("app.Service.ConsumePasswordReset: %w", err)
	}
	if err := s.Repo.MarkVerified(ctx, token.AccountID); err != nil {
		return fmt.Errorf("app.Service.ConsumePasswordReset: %w", err)
	}
	if err := s.SessionInvalidator.InvalidateAllForUser(ctx, token.AccountID); err != nil {
		return fmt.Errorf("app.Service.ConsumePasswordReset: %w", err)
	}
	return nil
}

func validateRegistration(cmd CreateCommand) (string, error) {
	fe := domain.FieldErrors{}
	normalizedEmail := validateRegistrationInvariants(cmd, fe)
	applyRegistrationPolicies(cmd, fe)

	if cmd.Password != cmd.PasswordConfirm {
		fe.Add("password_confirm", "passwords do not match")
	}

	if fe.Has() {
		return "", &domain.ValidationError{Fields: fe}
	}
	return normalizedEmail, nil
}

func validateRegistrationInvariants(cmd CreateCommand, fe domain.FieldErrors) string {
	if err := domain.ValidateUsername(cmd.Username); err != nil {
		fe.Add("username", err.Error())
	}
	normalizedEmail, emailErr := domain.ValidateEmail(cmd.Email)
	if emailErr != nil {
		fe.Add("email", emailErr.Error())
	}
	if err := domain.ValidatePassword(cmd.Password); err != nil {
		fe.Add("password", err.Error())
	}
	if err := domain.ValidateGender(cmd.Gender); err != nil {
		fe.Add("gender", err.Error())
	}
	return normalizedEmail
}

func applyRegistrationPolicies(cmd CreateCommand, fe domain.FieldErrors) {
	if fe["username"] == "" {
		if err := domain.CheckRegistrationUsername(cmd.Username); err != nil {
			fe.Add("username", err.Error())
		}
	}
	if fe["password"] == "" {
		if err := domain.CheckRegistrationPassword(cmd.Password); err != nil {
			fe.Add("password", err.Error())
		}
	}
}

func (s *Service) checkRegistrationUniqueness(ctx context.Context, username, email string) error {
	fe := domain.FieldErrors{}

	existing, err := s.Repo.GetByUsername(ctx, username)
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return fmt.Errorf("app.Service.Create: %w", err)
	}
	if existing != nil {
		fe.Add("username", "username already taken")
	}

	existing, err = s.Repo.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return fmt.Errorf("app.Service.Create: %w", err)
	}
	if existing != nil {
		fe.Add("email", "email already in use")
	}

	if fe.Has() {
		return &domain.ValidationError{Fields: fe}
	}
	return nil
}

func (s *Service) GetAll(ctx context.Context) ([]GetDTO, error) {
	allUsers, err := s.Repo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("app.Service.GetAll: %w", err)
	}

	var dtoList []GetDTO
	for _, userData := range allUsers {
		dtoList = append(dtoList, GetDTO{
			ID:       userData.ID,
			Username: userData.Username,
			Email:    userData.Email,
		})
	}

	return dtoList, nil
}

func (s *Service) GetByID(ctx context.Context, id int) (*GetDTO, error) {
	userData, err := s.Repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("app.Service.GetByID: %w", err)
	}

	return &GetDTO{
		ID:       userData.ID,
		Username: userData.Username,
		Email:    userData.Email,
	}, nil
}

func (s *Service) GetByEmail(ctx context.Context, email string) (*GetDTO, error) {
	userData, err := s.Repo.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("app.Service.GetByEmail: %w", err)
	}

	return &GetDTO{
		ID:       userData.ID,
		Username: userData.Username,
		Email:    userData.Email,
	}, nil
}

func (s *Service) Delete(ctx context.Context, id int) error {
	if err := s.Repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("app.Service.Delete: %w", err)
	}
	return nil
}

func (s *Service) PeekPasswordReset(ctx context.Context, rawToken string) (*actiontoken.ActionToken, error) {
	token, err := s.TokenManager.Peek(ctx, actiontoken.PasswordReset, rawToken)
	if err != nil {
		return nil, fmt.Errorf("app.Service.PeekPasswordReset: %w", err)
	}
	return token, nil
}

func (s *Service) Authenticate(ctx context.Context, cmd LoginCommand) (*GetDTO, error) {
	user, err := s.Repo.Authenticate(ctx, cmd.Username, cmd.Password)
	if errors.Is(err, domain.ErrInvalidCredentials) {
		return nil, domain.ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("app.Service.Authenticate: %w", err)
	}

	return &GetDTO{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	}, nil
}
