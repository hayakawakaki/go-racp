package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
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
		"INSERT INTO login (userid, email, user_pass, sex, birthdate, state) VALUES (?, ?, ?, ?, ?, 1)",
		user.Username, user.Email, user.Password, user.Gender, user.Birthdate)
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.Create: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.Create: %w", err)
	}
	user.ID = int(id)
	user.State = 1

	return user, nil
}

func (r *Repository) GetAll(ctx context.Context) ([]domain.User, error) {
	rows, err := r.Client.QueryContext(ctx,
		"SELECT account_id, userid, email, birthdate, state, group_id, unban_time FROM login")
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.GetAll: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var userList []domain.User
	for rows.Next() {
		var u domain.User
		var unbanSecs uint32
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Birthdate, &u.State, &u.GroupID, &unbanSecs); err != nil {
			return nil, fmt.Errorf("infra.Repository.GetAll: %w", err)
		}
		u.UnbanTime = unbanTimeFromSeconds(unbanSecs)
		userList = append(userList, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.Repository.GetAll: %w", err)
	}

	return userList, nil
}

func (r *Repository) GetByID(ctx context.Context, id int) (*domain.User, error) {
	var u domain.User
	var unbanSecs uint32
	err := r.Client.QueryRowContext(ctx,
		"SELECT account_id, userid, email, birthdate, state, group_id, unban_time FROM login WHERE account_id = ?", id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.Birthdate, &u.State, &u.GroupID, &unbanSecs)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.GetByID: %w", err)
	}
	u.UnbanTime = unbanTimeFromSeconds(unbanSecs)

	return &u, nil
}

func (r *Repository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	var u domain.User
	var unbanSecs uint32
	err := r.Client.QueryRowContext(ctx,
		"SELECT account_id, userid, email, birthdate, state, group_id, unban_time FROM login WHERE userid = ?", username,
	).Scan(&u.ID, &u.Username, &u.Email, &u.Birthdate, &u.State, &u.GroupID, &unbanSecs)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.GetByUsername: %w", err)
	}
	u.UnbanTime = unbanTimeFromSeconds(unbanSecs)

	return &u, nil
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	var unbanSecs uint32
	err := r.Client.QueryRowContext(ctx,
		"SELECT account_id, userid, email, birthdate, state, group_id, unban_time FROM login WHERE email = ?", email,
	).Scan(&u.ID, &u.Username, &u.Email, &u.Birthdate, &u.State, &u.GroupID, &unbanSecs)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.GetByEmail: %w", err)
	}
	u.UnbanTime = unbanTimeFromSeconds(unbanSecs)

	return &u, nil
}

func (r *Repository) Authenticate(ctx context.Context, username, password string) (*domain.User, error) {
	var u domain.User
	var unbanSecs uint32
	err := r.Client.QueryRowContext(ctx,
		"SELECT account_id, userid, email, birthdate, state, group_id, unban_time FROM login WHERE userid = ? AND user_pass = ?",
		username, password,
	).Scan(&u.ID, &u.Username, &u.Email, &u.Birthdate, &u.State, &u.GroupID, &unbanSecs)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.Authenticate: %w", err)
	}
	u.UnbanTime = unbanTimeFromSeconds(unbanSecs)

	return &u, nil
}

func (r *Repository) MarkVerified(ctx context.Context, accountID int) error {
	var exists int
	err := r.Client.QueryRowContext(ctx,
		"SELECT account_id FROM login WHERE account_id = ?",
		accountID,
	).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("infra.Repository.MarkVerified: %w", err)
	}

	if _, err := r.Client.ExecContext(ctx,
		"UPDATE login SET state = 0 WHERE account_id = ? AND state = 1",
		accountID,
	); err != nil {
		return fmt.Errorf("infra.Repository.MarkVerified: %w", err)
	}

	return nil
}

func (r *Repository) UpdatePassword(ctx context.Context, accountID int, newPassword string) error {
	res, err := r.Client.ExecContext(ctx,
		"UPDATE login SET user_pass = ? WHERE account_id = ?",
		newPassword, accountID,
	)
	if err != nil {
		return fmt.Errorf("infra.Repository.UpdatePassword: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("infra.Repository.UpdatePassword: %w", err)
	}
	if rows == 0 {
		return domain.ErrUserNotFound
	}

	return nil
}

func (r *Repository) UpdateEmail(ctx context.Context, accountID int, newEmail string) error {
	res, err := r.Client.ExecContext(ctx,
		"UPDATE login SET email = ? WHERE account_id = ?",
		newEmail, accountID,
	)
	if err != nil {
		return fmt.Errorf("infra.Repository.UpdateEmail: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("infra.Repository.UpdateEmail: %w", err)
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

func unbanTimeFromSeconds(secs uint32) time.Time {
	if secs == 0 {
		return time.Time{}
	}
	return time.Unix(int64(secs), 0)
}
