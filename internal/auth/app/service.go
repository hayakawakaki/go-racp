package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hayakawakaki/go-racp/internal/auth/domain"
	mailtemplate "github.com/hayakawakaki/go-racp/internal/infra/mailer/template"
)

type Mailer interface {
	SendAsync(to, subject, body string)
}

type VerificationConfig struct {
	AppURL         string
	ServerName     string
	TokenTTL       time.Duration
	ResendCooldown time.Duration
}

type Service struct {
	Repo      domain.Repository
	TokenRepo domain.TokenRepository
	Mail      Mailer
	now       func() time.Time
	cfg       VerificationConfig
	createMu  sync.Mutex
}

type Option func(*Service)

func WithVerification(tokenRepo domain.TokenRepository, mailer Mailer, cfg VerificationConfig) Option {
	if cfg.TokenTTL <= 0 {
		panic("auth: VerificationConfig.TokenTTL must be > 0")
	}
	if cfg.ResendCooldown <= 0 {
		cfg.ResendCooldown = 60 * time.Second
	}
	return func(s *Service) {
		s.TokenRepo = tokenRepo
		s.Mail = mailer
		s.cfg = cfg
	}
}

func NewService(repo domain.Repository, opts ...Option) *Service {
	s := &Service{
		Repo: repo,
		now:  time.Now,
	}
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

	s.createMu.Lock()
	defer s.createMu.Unlock()

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

	if s.TokenRepo != nil && s.Mail != nil {
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
	if err := s.TokenRepo.DeleteUnconsumed(ctx, accountID, domain.ActionEmailVerification); err != nil {
		return fmt.Errorf("app.Service.IssueVerification: %w", err)
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Errorf("app.Service.IssueVerification: %w", err)
	}
	hash := sha256.Sum256(raw[:])
	now := s.now()
	token := &domain.ActionToken{
		TokenHash: hash,
		AccountID: accountID,
		Action:    domain.ActionEmailVerification,
		ExpiresAt: now.Add(s.cfg.TokenTTL),
		CreatedAt: now,
	}
	if err := s.TokenRepo.Insert(ctx, token); err != nil {
		return fmt.Errorf("app.Service.IssueVerification: %w", err)
	}

	rawToken := base64.RawURLEncoding.EncodeToString(raw[:])
	url := strings.TrimRight(s.cfg.AppURL, "/") + "/verify?token=" + rawToken

	body, err := renderVerificationEmail(ctx, mailtemplate.VerificationData{
		ServerName: s.cfg.ServerName,
		Username:   username,
		URL:        url,
	})
	if err != nil {
		return fmt.Errorf("app.Service.IssueVerification: %w", err)
	}
	subject := s.cfg.ServerName + " - verify your email"
	s.Mail.SendAsync(email, subject, body)
	return nil
}

func (s *Service) ConsumeVerification(ctx context.Context, rawToken string) error {
	hash, ok := decodeActionToken(rawToken)
	if !ok {
		return domain.ErrTokenInvalid
	}
	token, err := s.TokenRepo.GetByHash(ctx, hash)
	if err != nil {
		return fmt.Errorf("app.Service.ConsumeVerification: %w", err)
	}
	if token.Action != domain.ActionEmailVerification {
		return domain.ErrTokenInvalid
	}
	if token.IsConsumed() {
		return domain.ErrTokenAlreadyUsed
	}
	now := s.now()
	if token.IsExpired(now) {
		return domain.ErrTokenExpired
	}
	if err := s.TokenRepo.MarkConsumed(ctx, hash, now); err != nil {
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
	last, err := s.TokenRepo.MostRecentIssuedAt(ctx, accountID, domain.ActionEmailVerification)
	if err != nil {
		return fmt.Errorf("app.Service.ResendVerification: %w", err)
	}
	if !last.IsZero() && s.now().Sub(last) < s.cfg.ResendCooldown {
		return nil
	}
	return s.IssueVerification(ctx, accountID, user.Email, user.Username)
}

func decodeActionToken(rawToken string) ([32]byte, bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(rawToken)
	if err != nil || len(decoded) != 32 {
		return [32]byte{}, false
	}
	return sha256.Sum256(decoded), true
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

func (s *Service) Update(ctx context.Context, id int, cmd UpdateCommand) (*GetDTO, error) {
	userData, err := s.Repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("app.Service.Update: %w", err)
	}

	normalizedEmail, err := domain.ValidateEmail(cmd.Email)
	if err != nil {
		return nil, &domain.ValidationError{Fields: domain.FieldErrors{"email": err.Error()}}
	}
	if pwErr := domain.ValidatePassword(cmd.Password); pwErr != nil {
		return nil, &domain.ValidationError{Fields: domain.FieldErrors{"password": pwErr.Error()}}
	}

	updated := &domain.User{
		ID:       userData.ID,
		Username: userData.Username,
		Email:    normalizedEmail,
		Password: cmd.Password,
		Gender:   userData.Gender,
	}

	updatedData, err := s.Repo.Update(ctx, updated)
	if err != nil {
		return nil, fmt.Errorf("app.Service.Update: %w", err)
	}

	return &GetDTO{
		ID:       updatedData.ID,
		Username: updatedData.Username,
		Email:    updatedData.Email,
	}, nil
}

func (s *Service) Delete(ctx context.Context, id int) error {
	if err := s.Repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("app.Service.Delete: %w", err)
	}
	return nil
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
