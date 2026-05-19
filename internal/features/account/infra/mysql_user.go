package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	domain2 "github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

type Repository struct {
	Client *sql.DB
}

func NewRepository(client *sql.DB) *Repository {
	return &Repository{
		Client: client,
	}
}

func (r *Repository) Create(ctx context.Context, user *domain2.User, password string) (*domain2.User, error) {
	res, err := r.Client.ExecContext(ctx,
		"INSERT INTO login (userid, email, user_pass, sex, birthdate, state) VALUES (?, ?, ?, ?, ?, 1)",
		user.Username, user.Email, password, user.Gender, user.Birthdate)
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

func (r *Repository) GetAll(ctx context.Context) ([]domain2.User, error) {
	rows, err := r.Client.QueryContext(ctx,
		"SELECT account_id, userid, email, birthdate, state, group_id, unban_time, last_ip, lastlogin FROM login")
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.GetAll: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var userList []domain2.User
	for rows.Next() {
		var (
			u         domain2.User
			unbanSecs uint32
			lastIP    sql.NullString
			lastLogin sql.NullTime
		)
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Birthdate, &u.State, &u.GroupID, &unbanSecs, &lastIP, &lastLogin); err != nil {
			return nil, fmt.Errorf("infra.Repository.GetAll: %w", err)
		}
		u.UnbanTime = unbanTimeFromSeconds(unbanSecs)
		u.LastIP = lastIP.String
		if lastLogin.Valid {
			u.LastLogin = lastLogin.Time
		}
		userList = append(userList, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.Repository.GetAll: %w", err)
	}

	return userList, nil
}

const selectLoginColumns = "account_id, userid, email, birthdate, state, group_id, unban_time, last_ip, lastlogin"

func (r *Repository) selectLoginRow(ctx context.Context, op, where string, args ...any) (*domain2.User, error) {
	var (
		u         domain2.User
		unbanSecs uint32
		lastIP    sql.NullString
		lastLogin sql.NullTime
	)
	err := r.Client.QueryRowContext(ctx,
		"SELECT "+selectLoginColumns+" FROM login WHERE "+where, args...,
	).Scan(&u.ID, &u.Username, &u.Email, &u.Birthdate, &u.State, &u.GroupID, &unbanSecs, &lastIP, &lastLogin)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain2.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.%s: %w", op, err)
	}
	u.UnbanTime = unbanTimeFromSeconds(unbanSecs)
	u.LastIP = lastIP.String
	if lastLogin.Valid {
		u.LastLogin = lastLogin.Time
	}

	return &u, nil
}

func (r *Repository) GetByID(ctx context.Context, id int) (*domain2.User, error) {
	return r.selectLoginRow(ctx, "GetByID", "account_id = ?", id)
}

func (r *Repository) GetByUsername(ctx context.Context, username string) (*domain2.User, error) {
	return r.selectLoginRow(ctx, "GetByUsername", "userid = ?", username)
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (*domain2.User, error) {
	return r.selectLoginRow(ctx, "GetByEmail", "email = ?", email)
}

func (r *Repository) VerifyPassword(ctx context.Context, id int, password string) (bool, error) {
	var stored string
	err := r.Client.QueryRowContext(ctx,
		"SELECT user_pass FROM login WHERE account_id = ?", id,
	).Scan(&stored)
	if errors.Is(err, sql.ErrNoRows) {
		return false, domain2.ErrUserNotFound
	}
	if err != nil {
		return false, fmt.Errorf("infra.Repository.VerifyPassword: %w", err)
	}

	return stored == password, nil
}

func (r *Repository) Authenticate(ctx context.Context, username, password string) (*domain2.User, error) {
	var (
		u         domain2.User
		unbanSecs uint32
		lastIP    sql.NullString
		lastLogin sql.NullTime
	)
	err := r.Client.QueryRowContext(ctx,
		"SELECT account_id, userid, email, birthdate, state, group_id, unban_time, last_ip, lastlogin FROM login WHERE userid = ? AND user_pass = ?",
		username, password,
	).Scan(&u.ID, &u.Username, &u.Email, &u.Birthdate, &u.State, &u.GroupID, &unbanSecs, &lastIP, &lastLogin)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain2.ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.Authenticate: %w", err)
	}
	u.UnbanTime = unbanTimeFromSeconds(unbanSecs)
	u.LastIP = lastIP.String
	if lastLogin.Valid {
		u.LastLogin = lastLogin.Time
	}

	return &u, nil
}

func (r *Repository) MarkVerified(ctx context.Context, accountID int) error {
	var exists int
	err := r.Client.QueryRowContext(ctx,
		"SELECT account_id FROM login WHERE account_id = ?",
		accountID,
	).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return domain2.ErrUserNotFound
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
		return domain2.ErrUserNotFound
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
		return domain2.ErrUserNotFound
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
		return domain2.ErrUserNotFound
	}

	return nil
}

const DefaultPerPage = 20

type ListQuery struct {
	Query     string
	Page      int
	PerPage   int
	ExcludeID int
}

type UserPage struct {
	Users      []domain2.User
	Total      int
	Page       int
	PerPage    int
	TotalPages int
}

func (r *Repository) List(ctx context.Context, q ListQuery) (UserPage, error) {
	if q.PerPage <= 0 {
		q.PerPage = DefaultPerPage
	}
	if q.Page <= 0 {
		q.Page = 1
	}

	where, args := buildListWhere(q)

	var total int
	if err := r.Client.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM login "+where, args...,
	).Scan(&total); err != nil {
		return UserPage{}, fmt.Errorf("infra.Repository.List count: %w", err)
	}

	offset := (q.Page - 1) * q.PerPage
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, q.PerPage, offset)

	rows, err := r.Client.QueryContext(ctx,
		"SELECT "+selectLoginColumns+" FROM login "+where+ //nolint:gosec // where built from constants
			" ORDER BY account_id ASC LIMIT ? OFFSET ?", queryArgs...,
	)
	if err != nil {
		return UserPage{}, fmt.Errorf("infra.Repository.List query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	users := make([]domain2.User, 0, q.PerPage)
	for rows.Next() {
		var (
			u         domain2.User
			unbanSecs uint32
			lastIP    sql.NullString
			lastLogin sql.NullTime
		)
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Birthdate, &u.State, &u.GroupID, &unbanSecs, &lastIP, &lastLogin); err != nil {
			return UserPage{}, fmt.Errorf("infra.Repository.List scan: %w", err)
		}
		u.UnbanTime = unbanTimeFromSeconds(unbanSecs)
		u.LastIP = lastIP.String
		if lastLogin.Valid {
			u.LastLogin = lastLogin.Time
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return UserPage{}, fmt.Errorf("infra.Repository.List rows: %w", err)
	}

	totalPages := (total + q.PerPage - 1) / q.PerPage

	return UserPage{Users: users, Total: total, Page: q.Page, PerPage: q.PerPage, TotalPages: totalPages}, nil
}

func buildListWhere(q ListQuery) (where string, args []any) {
	clauses := make([]string, 0, 2)
	if needle := strings.TrimSpace(q.Query); needle != "" {
		clauses = append(clauses, "(userid LIKE ? OR email LIKE ?)")
		like := "%" + needle + "%"
		args = append(args, like, like)
	}
	if q.ExcludeID > 0 {
		clauses = append(clauses, "account_id <> ?")
		args = append(args, q.ExcludeID)
	}
	if len(clauses) == 0 {
		return "", args
	}

	return "WHERE " + strings.Join(clauses, " AND "), args
}

func (r *Repository) UpdateBan(ctx context.Context, id, state int, unbanTime uint32) error {
	res, err := r.Client.ExecContext(ctx,
		"UPDATE login SET state = ?, unban_time = ? WHERE account_id = ?",
		state, unbanTime, id,
	)
	if err != nil {
		return fmt.Errorf("infra.Repository.UpdateBan: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("infra.Repository.UpdateBan rows: %w", err)
	}
	if rows == 0 {
		return domain2.ErrUserNotFound
	}

	return nil
}

func (r *Repository) UpdateGroup(ctx context.Context, id, groupID int) error {
	res, err := r.Client.ExecContext(ctx,
		"UPDATE login SET group_id = ? WHERE account_id = ?",
		groupID, id,
	)
	if err != nil {
		return fmt.Errorf("infra.Repository.UpdateGroup: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("infra.Repository.UpdateGroup rows: %w", err)
	}
	if rows == 0 {
		return domain2.ErrUserNotFound
	}

	return nil
}

func unbanTimeFromSeconds(secs uint32) time.Time {
	if secs == 0 {
		return time.Time{}
	}
	return time.Unix(int64(secs), 0)
}
