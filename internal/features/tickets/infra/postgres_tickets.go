package infra

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	domain2 "github.com/hayakawakaki/go-racp/internal/features/tickets/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const ticketColumns = `id, account_id, author_username, category, subject, status, last_actor, message_count, last_activity, closed_by, created_at`

type Repository struct {
	Pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{Pool: pool}
}

func (r *Repository) Create(ctx context.Context, ticket domain2.Ticket, initial domain2.Message) (int64, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("infra.Repository.Create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ticketID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO cp_tickets (account_id, author_username, category, subject, status, last_actor, message_count, last_activity, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, 1, $7, $8) RETURNING id`,
		ticket.AccountID, ticket.AuthorUsername, ticket.Category, ticket.Subject,
		string(ticket.Status), string(ticket.LastActor),
		ticket.LastActivity, ticket.CreatedAt,
	).Scan(&ticketID)
	if err != nil {
		return 0, fmt.Errorf("infra.Repository.Create: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO cp_ticket_messages (ticket_id, author_id, author_role, visibility, body, event, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		ticketID, initial.AuthorID, string(initial.AuthorRole),
		string(initial.Visibility), initial.Body, initial.Event, initial.CreatedAt,
	); err != nil {
		return 0, fmt.Errorf("infra.Repository.Create: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("infra.Repository.Create: %w", err)
	}

	return ticketID, nil
}

func scanTicket(row pgx.Row) (domain2.Ticket, error) {
	var (
		ticket    domain2.Ticket
		status    string
		lastActor string
		closedBy  *int
	)
	err := row.Scan(
		&ticket.ID, &ticket.AccountID, &ticket.AuthorUsername,
		&ticket.Category, &ticket.Subject,
		&status, &lastActor, &ticket.MessageCount,
		&ticket.LastActivity, &closedBy, &ticket.CreatedAt,
	)
	if err != nil {
		return domain2.Ticket{}, fmt.Errorf("infra.scanTicket: %w", err)
	}
	ticket.Status = domain2.Status(status)
	ticket.LastActor = domain2.Actor(lastActor)
	ticket.ClosedBy = closedBy

	return ticket, nil
}

func (r *Repository) Get(ctx context.Context, id int64) (domain2.Ticket, error) {
	row := r.Pool.QueryRow(ctx,
		`SELECT `+ticketColumns+` FROM cp_tickets WHERE id = $1`, id,
	)
	ticket, err := scanTicket(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain2.Ticket{}, domain2.ErrTicketNotFound
	}
	if err != nil {
		return domain2.Ticket{}, fmt.Errorf("infra.Repository.Get: %w", err)
	}

	return ticket, nil
}

func (r *Repository) ListForPlayer(ctx context.Context, accountID int, page domain2.Page) ([]domain2.Ticket, int, error) {
	var total int
	if err := r.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM cp_tickets WHERE account_id = $1`, accountID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("infra.Repository.ListForPlayer: %w", err)
	}

	rows, err := r.Pool.Query(ctx,
		`SELECT `+ticketColumns+` FROM cp_tickets
		 WHERE account_id = $1
		 ORDER BY last_activity DESC
		 LIMIT $2 OFFSET $3`,
		accountID, page.Limit, page.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("infra.Repository.ListForPlayer: %w", err)
	}
	defer rows.Close()

	out, err := collectTickets(rows)
	if err != nil {
		return nil, 0, fmt.Errorf("infra.Repository.ListForPlayer: %w", err)
	}

	return out, total, nil
}

func (r *Repository) ListForStaff(ctx context.Context, tab domain2.StaffTab, categoryKeys []string, page domain2.Page) ([]domain2.Ticket, int, error) {
	if len(categoryKeys) == 0 {
		return []domain2.Ticket{}, 0, nil
	}

	where, args := staffTabWhere(tab, categoryKeys)
	countQuery := `SELECT COUNT(*) FROM cp_tickets WHERE ` + where
	var total int
	if err := r.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("infra.Repository.ListForStaff: %w", err)
	}

	listArgs := append([]any{}, args...)
	listArgs = append(listArgs, page.Limit, page.Offset)
	listQuery := `SELECT ` + ticketColumns + ` FROM cp_tickets WHERE ` + where +
		` ORDER BY last_activity DESC LIMIT $` + strconv.Itoa(len(args)+1) +
		` OFFSET $` + strconv.Itoa(len(args)+2)

	rows, err := r.Pool.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("infra.Repository.ListForStaff: %w", err)
	}
	defer rows.Close()

	out, err := collectTickets(rows)
	if err != nil {
		return nil, 0, fmt.Errorf("infra.Repository.ListForStaff: %w", err)
	}

	return out, total, nil
}

func staffTabWhere(tab domain2.StaffTab, categoryKeys []string) (where string, args []any) {
	catList, catArgs := categoryInClause(categoryKeys, 1)
	base := `category IN (` + catList + `)`
	switch tab {
	case domain2.TabOpenNoResponse:
		return base + ` AND status = 'open' AND message_count = 1`, catArgs
	case domain2.TabActive:
		return base + ` AND status = 'open' AND message_count > 1`, catArgs
	case domain2.TabTerminal:
		return base + ` AND status IN ('resolved','closed')`, catArgs
	}

	return base, catArgs
}

func categoryInClause(keys []string, startAt int) (clause string, args []any) {
	args = make([]any, 0, len(keys))
	placeholders := make([]string, 0, len(keys))
	for i, key := range keys {
		placeholders = append(placeholders, "$"+strconv.Itoa(startAt+i))
		args = append(args, key)
	}

	return strings.Join(placeholders, ","), args
}

func collectTickets(rows pgx.Rows) ([]domain2.Ticket, error) {
	out := []domain2.Ticket{}
	for rows.Next() {
		ticket, err := scanTicket(rows)
		if err != nil {
			return nil, fmt.Errorf("infra.collectTickets: %w", err)
		}
		out = append(out, ticket)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.collectTickets: %w", err)
	}

	return out, nil
}

func (r *Repository) CountOpenForPlayer(ctx context.Context, accountID int) (int, error) {
	var count int
	err := r.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM cp_tickets WHERE account_id = $1 AND status = 'open'`, accountID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("infra.Repository.CountOpenForPlayer: %w", err)
	}

	return count, nil
}

func (r *Repository) MostRecentOpenedAt(ctx context.Context, accountID int) (time.Time, error) {
	var at *time.Time
	err := r.Pool.QueryRow(ctx,
		`SELECT MAX(created_at) FROM cp_tickets WHERE account_id = $1`, accountID,
	).Scan(&at)
	if err != nil {
		return time.Time{}, fmt.Errorf("infra.Repository.MostRecentOpenedAt: %w", err)
	}
	if at == nil {
		return time.Time{}, nil
	}

	return *at, nil
}

func (r *Repository) AppendPublicMessage(ctx context.Context, ticketID int64, message domain2.Message) (domain2.Ticket, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return domain2.Ticket{}, fmt.Errorf("infra.Repository.AppendPublicMessage: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, execErr := tx.Exec(ctx,
		`INSERT INTO cp_ticket_messages (ticket_id, author_id, author_role, visibility, body, created_at)
		 VALUES ($1, $2, $3, 'public', $4, $5)`,
		ticketID, message.AuthorID, string(message.AuthorRole), message.Body, message.CreatedAt,
	); execErr != nil {
		return domain2.Ticket{}, fmt.Errorf("infra.Repository.AppendPublicMessage: %w", execErr)
	}

	row := tx.QueryRow(ctx,
		`UPDATE cp_tickets
		 SET last_actor = $1, message_count = message_count + 1, last_activity = $2
		 WHERE id = $3
		 RETURNING `+ticketColumns,
		string(message.AuthorRole), message.CreatedAt, ticketID,
	)
	updated, err := scanTicket(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain2.Ticket{}, domain2.ErrTicketNotFound
	}
	if err != nil {
		return domain2.Ticket{}, fmt.Errorf("infra.Repository.AppendPublicMessage: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain2.Ticket{}, fmt.Errorf("infra.Repository.AppendPublicMessage: %w", err)
	}

	return updated, nil
}

func (r *Repository) AppendInternalNote(ctx context.Context, ticketID int64, message domain2.Message) error {
	_, err := r.Pool.Exec(ctx,
		`INSERT INTO cp_ticket_messages (ticket_id, author_id, author_role, visibility, body, created_at)
		 VALUES ($1, $2, 'staff', 'internal', $3, $4)`,
		ticketID, message.AuthorID, message.Body, message.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("infra.Repository.AppendInternalNote: %w", err)
	}

	return nil
}

func (r *Repository) AppendSystemEvent(ctx context.Context, ticketID int64, updated domain2.Ticket, message domain2.Message) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("infra.Repository.AppendSystemEvent: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`INSERT INTO cp_ticket_messages (ticket_id, author_id, author_role, visibility, body, event, created_at)
		 VALUES ($1, $2, 'staff', 'system', $3, $4, $5)`,
		ticketID, message.AuthorID, message.Body, message.Event, message.CreatedAt,
	); err != nil {
		return fmt.Errorf("infra.Repository.AppendSystemEvent: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE cp_tickets
		 SET category = $1, subject = $2, last_activity = $3
		 WHERE id = $4`,
		updated.Category, updated.Subject, updated.LastActivity, ticketID,
	); err != nil {
		return fmt.Errorf("infra.Repository.AppendSystemEvent: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("infra.Repository.AppendSystemEvent: %w", err)
	}

	return nil
}

func (r *Repository) SetTerminal(ctx context.Context, ticketID int64, status domain2.Status, staffID int, at time.Time) (domain2.Ticket, domain2.Message, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return domain2.Ticket{}, domain2.Message{}, fmt.Errorf("infra.Repository.SetTerminal: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	eventType := "resolve"
	fallback := "Ticket resolved"
	if status == domain2.StatusClosed {
		eventType = "close"
		fallback = "Ticket closed"
	}
	eventJSON := []byte(`{"type":"` + eventType + `"}`)

	var messageID int64
	if insertErr := tx.QueryRow(ctx,
		`INSERT INTO cp_ticket_messages (ticket_id, author_id, author_role, visibility, body, event, created_at)
		 VALUES ($1, $2, 'staff', 'system', $3, $4, $5) RETURNING id`,
		ticketID, staffID, fallback, eventJSON, at,
	).Scan(&messageID); insertErr != nil {
		return domain2.Ticket{}, domain2.Message{}, fmt.Errorf("infra.Repository.SetTerminal: %w", insertErr)
	}

	row := tx.QueryRow(ctx,
		`UPDATE cp_tickets
		 SET status = $1, closed_by = $2, last_activity = $3
		 WHERE id = $4
		 RETURNING `+ticketColumns,
		string(status), staffID, at, ticketID,
	)
	updated, err := scanTicket(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain2.Ticket{}, domain2.Message{}, domain2.ErrTicketNotFound
	}
	if err != nil {
		return domain2.Ticket{}, domain2.Message{}, fmt.Errorf("infra.Repository.SetTerminal: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain2.Ticket{}, domain2.Message{}, fmt.Errorf("infra.Repository.SetTerminal: %w", err)
	}

	return updated, domain2.Message{
		ID:         messageID,
		TicketID:   ticketID,
		AuthorID:   staffID,
		AuthorRole: domain2.ActorStaff,
		Visibility: domain2.VisibilitySystem,
		Body:       fallback,
		Event:      eventJSON,
		CreatedAt:  at,
	}, nil
}
