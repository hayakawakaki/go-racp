package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/hayakawakaki/go-racp/internal/auth/domain"
)

type Repository struct {
	Client *sql.DB
}

func NewRepository(client *sql.DB) *Repository {
	return &Repository{
		Client: client,
	}
}

func (r *Repository) Create(ctx context.Context, user *domain.User) (*domain.User, error) {
	res, err := r.Client.ExecContext(ctx,
		"INSERT INTO login (userid, email, user_pass, sex, group_id) VALUES (?, ?, ?, ?, 5)",
		user.Username, user.Email, user.Password, user.Gender)
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.Create: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.Create: %w", err)
	}
	user.ID = int(id)
	user.GroupID = 5

	return user, nil
}

func (r *Repository) GetAll(ctx context.Context) ([]domain.User, error) {
	rows, err := r.Client.QueryContext(ctx,
		"SELECT account_id, userid, email, group_id FROM login")
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.GetAll: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var userList []domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.GroupID); err != nil {
			return nil, fmt.Errorf("infra.Repository.GetAll: %w", err)
		}
		userList = append(userList, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.Repository.GetAll: %w", err)
	}

	return userList, nil
}

func (r *Repository) GetByID(ctx context.Context, id int) (*domain.User, error) {
	var u domain.User
	err := r.Client.QueryRowContext(ctx,
		"SELECT account_id, userid, email, group_id FROM login WHERE account_id = ?", id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.GroupID)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.GetByID: %w", err)
	}
	return &u, nil
}

func (r *Repository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	var u domain.User
	err := r.Client.QueryRowContext(ctx,
		"SELECT account_id, userid, email, group_id FROM login WHERE userid = ?", username,
	).Scan(&u.ID, &u.Username, &u.Email, &u.GroupID)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.GetByUsername: %w", err)
	}
	return &u, nil
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	err := r.Client.QueryRowContext(ctx,
		"SELECT account_id, userid, email, group_id FROM login WHERE email = ?", email,
	).Scan(&u.ID, &u.Username, &u.Email, &u.GroupID)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.GetByEmail: %w", err)
	}
	return &u, nil
}

func (r *Repository) Update(ctx context.Context, user *domain.User) (*domain.User, error) {
	res, err := r.Client.ExecContext(ctx,
		"UPDATE login SET email = ?, user_pass = ? WHERE account_id = ?",
		user.Email, user.Password, user.ID)
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.Update: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.Update: %w", err)
	}
	if rows == 0 {
		return nil, domain.ErrUserNotFound
	}

	return user, nil
}

func (r *Repository) Authenticate(ctx context.Context, username, password string) (*domain.User, error) {
	var u domain.User
	err := r.Client.QueryRowContext(ctx,
		"SELECT account_id, userid, email, group_id FROM login WHERE userid = ? AND user_pass = ?",
		username, password,
	).Scan(&u.ID, &u.Username, &u.Email, &u.GroupID)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.Authenticate: %w", err)
	}
	return &u, nil
}

func (r *Repository) MarkVerified(ctx context.Context, accountID int) error {
	res, err := r.Client.ExecContext(ctx,
		"UPDATE login SET group_id = 0 WHERE account_id = ? AND group_id = 5",
		accountID,
	)
	if err != nil {
		return fmt.Errorf("infra.Repository.MarkVerified: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("infra.Repository.MarkVerified: %w", err)
	}
	if rows == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id int) error {
	res, err := r.Client.ExecContext(ctx,
		"DELETE FROM login WHERE account_id = ?", id)
	if err != nil {
		return fmt.Errorf("infra.Repository.Delete: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("infra.Repository.Delete: %w", err)
	}
	if rows == 0 {
		return domain.ErrUserNotFound
	}

	return nil
}
