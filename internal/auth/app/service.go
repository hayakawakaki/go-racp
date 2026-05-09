package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/hayakawakaki/go-racp/internal/auth/domain"
)

type Service struct {
	Repo domain.Repository
}

func NewService(repo domain.Repository) *Service {
	return &Service{
		Repo: repo,
	}
}

func (s *Service) Create(ctx context.Context, cmd CreateCommand) (*GetDTO, error) {
	normalizedEmail, err := validateRegistration(cmd)
	if err != nil {
		return nil, err
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

	return &GetDTO{
		ID:       created.ID,
		Username: created.Username,
		Email:    created.Email,
	}, nil
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
