package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/actiontoken"
	mailtemplate "github.com/hayakawakaki/go-racp/internal/infra/mailer/template"
)

const (
	fieldUsername    = "username"
	fieldPassword    = "password"
	fieldNewEmail    = "new_email"
	fieldNewPassword = "new_password"
)

type Mailer interface {
	SendAsync(to, subject, body string)
}

type SessionInvalidator interface {
	InvalidateAll(ctx context.Context, userID int) error
	InvalidateOthers(ctx context.Context, userID int, currentRawToken string) error
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

type EmailChangeConfig struct {
	AppURL           string
	ServerName       string
	TokenTTL         time.Duration
	RequestCooldown  time.Duration
	ChangeCooldown   time.Duration
	PasswordCooldown time.Duration
}

type Service struct {
	TokenManager       *actiontoken.Manager
	now                func() time.Time
	Repo               domain.Repository
	ChangeLog          domain.ChangeLog
	Mail               Mailer
	SessionInvalidator SessionInvalidator
	verifyCfg          VerificationConfig
	resetCfg           PasswordResetConfig
	emailCfg           EmailChangeConfig
	emailUniqueMu      sync.Mutex
	enableVerify       bool
	enableReset        bool
	enableEmailChange  bool
}

type Option func(*Service)

func WithVerification(manager *actiontoken.Manager, mail Mailer, cfg VerificationConfig) Option {
	if cfg.TokenTTL <= 0 {
		panic("account: VerificationConfig.TokenTTL must be > 0")
	}
	if cfg.ResendCooldown <= 0 {
		panic("account: VerificationConfig.ResendCooldown must be > 0")
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
		panic("account: PasswordResetConfig.TokenTTL must be > 0")
	}
	if cfg.ResendCooldown <= 0 {
		panic("account: PasswordResetConfig.ResendCooldown must be > 0")
	}
	if cfg.ChangeCooldown <= 0 {
		panic("account: PasswordResetConfig.ChangeCooldown must be > 0")
	}
	return func(s *Service) {
		s.TokenManager = manager
		s.Mail = mail
		s.resetCfg = cfg
		s.enableReset = true
	}
}

func WithEmailChange(manager *actiontoken.Manager, mail Mailer, cfg EmailChangeConfig) Option {
	if cfg.TokenTTL <= 0 {
		panic("account: EmailChangeConfig.TokenTTL must be > 0")
	}
	if cfg.RequestCooldown <= 0 {
		panic("account: EmailChangeConfig.RequestCooldown must be > 0")
	}
	if cfg.ChangeCooldown <= 0 {
		panic("account: EmailChangeConfig.ChangeCooldown must be > 0")
	}
	if cfg.PasswordCooldown <= 0 {
		panic("account: EmailChangeConfig.PasswordCooldown must be > 0")
	}
	return func(s *Service) {
		s.TokenManager = manager
		s.Mail = mail
		s.emailCfg = cfg
		s.enableEmailChange = true
	}
}

func WithChangeLog(log domain.ChangeLog) Option {
	return func(s *Service) { s.ChangeLog = log }
}

func WithSessionInvalidator(invalidator SessionInvalidator) Option {
	return func(s *Service) { s.SessionInvalidator = invalidator }
}

//nolint:cyclop // linear nil-check would flag this, splitting would not improve readability
func NewService(repo domain.Repository, opts ...Option) *Service {
	if repo == nil {
		panic("account: NewService: repo must not be nil")
	}
	s := &Service{Repo: repo, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	if s.enableVerify {
		if s.TokenManager == nil {
			panic("account: WithVerification requires a non-nil TokenManager")
		}
		if s.Mail == nil {
			panic("account: WithVerification requires a non-nil Mailer")
		}
	}
	if s.enableReset {
		if s.TokenManager == nil {
			panic("account: WithPasswordReset requires a non-nil TokenManager")
		}
		if s.Mail == nil {
			panic("account: WithPasswordReset requires a non-nil Mailer")
		}
		if s.ChangeLog == nil {
			panic("account: WithPasswordReset requires WithChangeLog")
		}
		if s.SessionInvalidator == nil {
			panic("account: WithPasswordReset requires WithSessionInvalidator")
		}
	}
	if s.enableEmailChange {
		if s.TokenManager == nil {
			panic("account: WithEmailChange requires a non-nil TokenManager")
		}
		if s.Mail == nil {
			panic("account: WithEmailChange requires a non-nil Mailer")
		}
		if s.ChangeLog == nil {
			panic("account: WithEmailChange requires WithChangeLog")
		}
		if s.SessionInvalidator == nil {
			panic("account: WithEmailChange requires WithSessionInvalidator")
		}
	}
	return s
}

func (s *Service) Create(ctx context.Context, cmd CreateCommand) (*GetDTO, error) {
	normalizedEmail, err := validateRegistration(cmd)
	if err != nil {
		return nil, err
	}

	s.emailUniqueMu.Lock()
	defer s.emailUniqueMu.Unlock()

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
	lastChange, err := s.ChangeLog.MostRecent(ctx, user.ID, domain.ChangeTypePassword)
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
		return &domain.ValidationError{Fields: domain.FieldErrors{fieldPassword: err.Error()}}
	}
	if err := domain.CheckRegistrationPassword(newPassword); err != nil {
		return &domain.ValidationError{Fields: domain.FieldErrors{fieldPassword: err.Error()}}
	}
	token, err := s.TokenManager.Consume(ctx, actiontoken.PasswordReset, rawToken)
	if err != nil {
		return fmt.Errorf("app.Service.ConsumePasswordReset: %w", err)
	}
	if err := s.Repo.UpdatePassword(ctx, token.AccountID, newPassword); err != nil {
		return fmt.Errorf("app.Service.ConsumePasswordReset: %w", err)
	}
	if err := s.ChangeLog.Record(ctx, token.AccountID, domain.ChangeTypePassword, s.now()); err != nil {
		return fmt.Errorf("app.Service.ConsumePasswordReset: %w", err)
	}
	if err := s.Repo.MarkVerified(ctx, token.AccountID); err != nil {
		return fmt.Errorf("app.Service.ConsumePasswordReset: %w", err)
	}
	if err := s.SessionInvalidator.InvalidateAll(ctx, token.AccountID); err != nil {
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
		fe.Add(fieldUsername, err.Error())
	}
	normalizedEmail, emailErr := domain.ValidateEmail(cmd.Email)
	if emailErr != nil {
		fe.Add("email", emailErr.Error())
	}
	if err := domain.ValidatePassword(cmd.Password); err != nil {
		fe.Add(fieldPassword, err.Error())
	}
	if err := domain.ValidateGender(cmd.Gender); err != nil {
		fe.Add("gender", err.Error())
	}
	return normalizedEmail
}

func applyRegistrationPolicies(cmd CreateCommand, fe domain.FieldErrors) {
	if fe[fieldUsername] == "" {
		if err := domain.CheckRegistrationUsername(cmd.Username); err != nil {
			fe.Add(fieldUsername, err.Error())
		}
	}
	if fe[fieldPassword] == "" {
		if err := domain.CheckRegistrationPassword(cmd.Password); err != nil {
			fe.Add(fieldPassword, err.Error())
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
		fe.Add(fieldUsername, "username already taken")
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

func (s *Service) PeekVerification(ctx context.Context, rawToken string) (*actiontoken.ActionToken, error) {
	token, err := s.TokenManager.Peek(ctx, actiontoken.EmailVerification, rawToken)
	if err != nil {
		return nil, fmt.Errorf("app.Service.PeekVerification: %w", err)
	}
	return token, nil
}

func (s *Service) PeekEmailChange(ctx context.Context, rawToken string) (*actiontoken.ActionToken, error) {
	token, err := s.TokenManager.Peek(ctx, actiontoken.EmailChange, rawToken)
	if err != nil {
		return nil, fmt.Errorf("app.Service.PeekEmailChange: %w", err)
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
	if err := domain.ValidatePassword(newPassword); err != nil {
		return &domain.ValidationError{Fields: domain.FieldErrors{fieldNewPassword: err.Error()}}
	}
	if err := domain.CheckRegistrationPassword(newPassword); err != nil {
		return &domain.ValidationError{Fields: domain.FieldErrors{fieldNewPassword: err.Error()}}
	}
	if newPassword != confirmPassword {
		return &domain.ValidationError{Fields: domain.FieldErrors{"new_password_confirm": "passwords do not match"}}
	}
	user, err := s.Repo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("app.Service.UpdatePassword: %w", err)
	}
	if user.Password != currentPassword {
		return &domain.ValidationError{Fields: domain.FieldErrors{"current_password": "incorrect"}}
	}
	lastChange, err := s.ChangeLog.MostRecent(ctx, userID, domain.ChangeTypePassword)
	if err != nil {
		return fmt.Errorf("app.Service.UpdatePassword: %w", err)
	}
	if !lastChange.IsZero() && s.now().Sub(lastChange) < s.emailCfg.PasswordCooldown {
		return domain.ErrPasswordRecentlyChanged
	}
	if err := s.Repo.UpdatePassword(ctx, userID, newPassword); err != nil {
		return fmt.Errorf("app.Service.UpdatePassword: %w", err)
	}
	if err := s.ChangeLog.Record(ctx, userID, domain.ChangeTypePassword, s.now()); err != nil {
		return fmt.Errorf("app.Service.UpdatePassword: %w", err)
	}
	if err := s.SessionInvalidator.InvalidateOthers(ctx, userID, currentRawToken); err != nil {
		return fmt.Errorf("app.Service.UpdatePassword: %w", err)
	}
	return nil
}

//nolint:cyclop // sequential validation, splitting would break the flow
func (s *Service) RequestEmailChange(ctx context.Context, userID int, currentPassword, newEmail string) error {
	normalizedNewEmail, err := domain.ValidateEmail(newEmail)
	if err != nil {
		return &domain.ValidationError{Fields: domain.FieldErrors{fieldNewEmail: err.Error()}}
	}
	user, err := s.Repo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("app.Service.RequestEmailChange: %w", err)
	}
	if user.Password != currentPassword {
		return &domain.ValidationError{Fields: domain.FieldErrors{"current_password": "incorrect"}}
	}
	if strings.EqualFold(normalizedNewEmail, user.Email) {
		return &domain.ValidationError{Fields: domain.FieldErrors{fieldNewEmail: "same as current"}}
	}
	existing, err := s.Repo.GetByEmail(ctx, normalizedNewEmail)
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return fmt.Errorf("app.Service.RequestEmailChange: %w", err)
	}
	if existing != nil && existing.ID != userID {
		return &domain.ValidationError{Fields: domain.FieldErrors{fieldNewEmail: "already in use"}}
	}
	lastChange, err := s.ChangeLog.MostRecent(ctx, userID, domain.ChangeTypeEmail)
	if err != nil {
		return fmt.Errorf("app.Service.RequestEmailChange: %w", err)
	}
	if !lastChange.IsZero() && s.now().Sub(lastChange) < s.emailCfg.ChangeCooldown {
		return domain.ErrEmailRecentlyChanged
	}
	last, err := s.TokenManager.MostRecentIssuedAt(ctx, userID, actiontoken.EmailChange)
	if err != nil {
		return fmt.Errorf("app.Service.RequestEmailChange: %w", err)
	}
	if !last.IsZero() && s.now().Sub(last) < s.emailCfg.RequestCooldown {
		return ErrEmailChangeCooldown
	}
	raw, err := s.TokenManager.Issue(ctx, actiontoken.EmailChange, userID, []byte(normalizedNewEmail), s.emailCfg.TokenTTL)
	if err != nil {
		return fmt.Errorf("app.Service.RequestEmailChange: %w", err)
	}
	url := strings.TrimRight(s.emailCfg.AppURL, "/") + "/verify-email-change?token=" + raw
	body, err := renderEmailChangeEmail(ctx, mailtemplate.EmailChangeData{
		ServerName: s.emailCfg.ServerName,
		Username:   user.Username,
		NewEmail:   normalizedNewEmail,
		URL:        url,
	})
	if err != nil {
		return fmt.Errorf("app.Service.RequestEmailChange: %w", err)
	}
	subject := s.emailCfg.ServerName + " - confirm your new email"
	s.Mail.SendAsync(normalizedNewEmail, subject, body)
	return nil
}

func (s *Service) ConsumeEmailChange(ctx context.Context, rawToken string) (*domain.User, error) {
	token, err := s.TokenManager.Consume(ctx, actiontoken.EmailChange, rawToken)
	if err != nil {
		return nil, fmt.Errorf("app.Service.ConsumeEmailChange: %w", err)
	}
	newEmail := string(token.Payload)
	if _, validateErr := domain.ValidateEmail(newEmail); validateErr != nil {
		return nil, actiontoken.ErrTokenInvalid
	}

	s.emailUniqueMu.Lock()
	defer s.emailUniqueMu.Unlock()

	existing, err := s.Repo.GetByEmail(ctx, newEmail)
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return nil, fmt.Errorf("app.Service.ConsumeEmailChange: %w", err)
	}
	if existing != nil && existing.ID != token.AccountID {
		return nil, domain.ErrEmailTaken
	}
	if updateErr := s.Repo.UpdateEmail(ctx, token.AccountID, newEmail); updateErr != nil {
		return nil, fmt.Errorf("app.Service.ConsumeEmailChange: %w", updateErr)
	}
	if recordErr := s.ChangeLog.Record(ctx, token.AccountID, domain.ChangeTypeEmail, s.now()); recordErr != nil {
		return nil, fmt.Errorf("app.Service.ConsumeEmailChange: %w", recordErr)
	}
	user, err := s.Repo.GetByID(ctx, token.AccountID)
	if err != nil {
		return nil, fmt.Errorf("app.Service.ConsumeEmailChange: %w", err)
	}
	return user, nil
}
