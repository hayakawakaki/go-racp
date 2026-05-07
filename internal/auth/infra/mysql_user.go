// Package infra provides infrastructure-layer implementations of domain
// interfaces for the auth feature, backed by a MySQL database.
package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/hayakawakaki/go-racp/internal/auth/domain"
)

// Repository is the MySQL-backed implementation of domain.Repository.
// It maps domain.User fields to the `login` table columns.
type Repository struct {
	Client *sql.DB
}

// NewRepository creates a Repository that executes queries against client.
func NewRepository(client *sql.DB) *Repository {
	return &Repository{
		Client: client,
	}
}

// Create inserts a new row into the login table and sets user.ID from the
// auto-generated primary key before returning the record.
func (r *Repository) Create(ctx context.Context, user *domain.User) (*domain.User, error) {
	res, err := r.Client.ExecContext(ctx,
		"INSERT INTO login (userid, email, user_pass, sex) VALUES (?, ?, ?, ?)",
		user.Username, user.Email, user.Password, user.Gender)
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.Create: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.Create: %w", err)
	}
	user.ID = int(id)

	return user, nil
}

// GetAll retrieves all rows from the login table and returns them as a slice
// of domain.User values.
func (r *Repository) GetAll(ctx context.Context) ([]domain.User, error) {
	rows, err := r.Client.QueryContext(ctx,
		"SELECT account_id, userid, email FROM login")
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.GetAll: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var userList []domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email); err != nil {
			return nil, fmt.Errorf("infra.Repository.GetAll: %w", err)
		}
		userList = append(userList, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.Repository.GetAll: %w", err)
	}

	return userList, nil
}

// GetByID returns the user whose account_id matches id, or
// domain.ErrUserNotFound when no row is found.
func (r *Repository) GetByID(ctx context.Context, id int) (*domain.User, error) {
	var u domain.User
	err := r.Client.QueryRowContext(ctx,
		"SELECT account_id, userid, email FROM login WHERE account_id = ?", id,
	).Scan(&u.ID, &u.Username, &u.Email)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.GetByID: %w", err)
	}
	return &u, nil
}

// GetByUsername returns the user whose userid column matches username, or
// domain.ErrUserNotFound when no row is found.
func (r *Repository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	var u domain.User
	err := r.Client.QueryRowContext(ctx,
		"SELECT account_id, userid, email FROM login WHERE userid = ?", username,
	).Scan(&u.ID, &u.Username, &u.Email)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.GetByUsername: %w", err)
	}
	return &u, nil
}

// GetByEmail returns the user whose email column matches email, or
// domain.ErrUserNotFound when no row is found.
func (r *Repository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	err := r.Client.QueryRowContext(ctx,
		"SELECT account_id, userid, email FROM login WHERE email = ?", email,
	).Scan(&u.ID, &u.Username, &u.Email)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.GetByEmail: %w", err)
	}
	return &u, nil
}

// Update overwrites the email and user_pass columns for the row identified by
// user.ID. Returns domain.ErrUserNotFound when no row is affected.
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

// Delete removes the row with the given account_id. Returns
// domain.ErrUserNotFound when no row is affected.
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
