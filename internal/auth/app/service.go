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
	existing, err := s.Repo.GetByUsername(ctx, cmd.Username)
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return nil, fmt.Errorf("app.Service.Create: %w", err)
	}
	if existing != nil {
		return nil, domain.ErrUsernameConflict
	}

	existing, err = s.Repo.GetByEmail(ctx, cmd.Email)
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return nil, fmt.Errorf("app.Service.Create: %w", err)
	}
	if existing != nil {
		return nil, domain.ErrEmailConflict
	}

	newUser := domain.User{
		Username: cmd.Username,
		Password: cmd.Password,
		Email:    cmd.Email,
		Gender:   cmd.Gender,
	}

	createdUser, err := s.Repo.Create(ctx, &newUser)
	if err != nil {
		return nil, fmt.Errorf("app.Service.Create: %w", err)
	}

	return &GetDTO{
		ID:       createdUser.ID,
		Username: createdUser.Username,
		Email:    createdUser.Email,
	}, nil
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

	updatedUser := domain.User{
		ID:       userData.ID,
		Username: userData.Username,
		Email:    cmd.Email,
		Password: cmd.Password,
	}

	updatedData, err := s.Repo.Update(ctx, &updatedUser)
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
