package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hayakawakaki/go-racp/internal/users/domain"
)

const DefaultPerPage = 20

type ListQuery struct {
	Query   string
	Page    int
	PerPage int
}

type UserPage struct {
	Users      []domain.User
	Total      int
	Page       int
	PerPage    int
	TotalPages int
}

type UserRepository struct {
	Client *sql.DB
}

func NewUserRepository(client *sql.DB) *UserRepository {
	return &UserRepository{Client: client}
}

func (r *UserRepository) GetByID(ctx context.Context, id int) (*domain.User, error) {
	var (
		user      domain.User
		unbanSecs uint32
		lastIP    sql.NullString
		lastLogin sql.NullTime
	)
	err := r.Client.QueryRowContext(ctx,
		"SELECT account_id, userid, email, group_id, state, unban_time, last_ip, lastlogin FROM login WHERE account_id = ?", id,
	).Scan(&user.ID, &user.Username, &user.Email, &user.GroupID, &user.State, &unbanSecs, &lastIP, &lastLogin)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("infra.UserRepository.GetByID: %w", err)
	}
	user.UnbanTime = unbanTimeFromSeconds(unbanSecs)
	user.LastIP = lastIP.String
	if lastLogin.Valid {
		user.LastLogin = lastLogin.Time
	}

	return &user, nil
}

func (r *UserRepository) List(ctx context.Context, q ListQuery) (UserPage, error) {
	if q.PerPage <= 0 {
		q.PerPage = DefaultPerPage
	}
	if q.Page <= 0 {
		q.Page = 1
	}

	where := ""
	args := []any{}
	if needle := strings.TrimSpace(q.Query); needle != "" {
		where = "WHERE userid LIKE ? OR email LIKE ?"
		like := "%" + needle + "%"
		args = append(args, like, like)
	}

	var total int
	if err := r.Client.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM login "+where, args...,
	).Scan(&total); err != nil {
		return UserPage{}, fmt.Errorf("infra.UserRepository.List count: %w", err)
	}

	offset := (q.Page - 1) * q.PerPage
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, q.PerPage, offset)

	rows, err := r.Client.QueryContext(ctx,
		"SELECT account_id, userid, email, group_id, state, unban_time, last_ip, lastlogin FROM login "+where+
			" ORDER BY account_id ASC LIMIT ? OFFSET ?", queryArgs...,
	)
	if err != nil {
		return UserPage{}, fmt.Errorf("infra.UserRepository.List query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	users := make([]domain.User, 0, q.PerPage)
	for rows.Next() {
		var (
			u         domain.User
			unbanSecs uint32
			lastIP    sql.NullString
			lastLogin sql.NullTime
		)
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.GroupID, &u.State, &unbanSecs, &lastIP, &lastLogin); err != nil {
			return UserPage{}, fmt.Errorf("infra.UserRepository.List scan: %w", err)
		}
		u.UnbanTime = unbanTimeFromSeconds(unbanSecs)
		u.LastIP = lastIP.String
		if lastLogin.Valid {
			u.LastLogin = lastLogin.Time
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return UserPage{}, fmt.Errorf("infra.UserRepository.List rows: %w", err)
	}

	totalPages := (total + q.PerPage - 1) / q.PerPage

	return UserPage{Users: users, Total: total, Page: q.Page, PerPage: q.PerPage, TotalPages: totalPages}, nil
}

func (r *UserRepository) UpdateBan(ctx context.Context, id, state int, unbanTime uint32) error {
	res, err := r.Client.ExecContext(ctx,
		"UPDATE login SET state = ?, unban_time = ? WHERE account_id = ?",
		state, unbanTime, id,
	)
	if err != nil {
		return fmt.Errorf("infra.UserRepository.UpdateBan: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("infra.UserRepository.UpdateBan rows: %w", err)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *UserRepository) UpdateGroup(ctx context.Context, id, groupID int) error {
	res, err := r.Client.ExecContext(ctx,
		"UPDATE login SET group_id = ? WHERE account_id = ?",
		groupID, id,
	)
	if err != nil {
		return fmt.Errorf("infra.UserRepository.UpdateGroup: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("infra.UserRepository.UpdateGroup rows: %w", err)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func unbanTimeFromSeconds(secs uint32) time.Time {
	if secs == 0 {
		return time.Time{}
	}

	return time.Unix(int64(secs), 0)
}
