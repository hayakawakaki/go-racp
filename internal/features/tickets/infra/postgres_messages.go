package infra

import (
	"context"
	"fmt"

	"github.com/hayakawakaki/go-racp/internal/features/tickets/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageRepository struct {
	Pool *pgxpool.Pool
}

func NewMessageRepository(pool *pgxpool.Pool) *MessageRepository {
	return &MessageRepository{Pool: pool}
}

func (r *MessageRepository) List(ctx context.Context, ticketID int64, includeInternal bool) ([]domain.Message, error) {
	query := `SELECT id, ticket_id, author_id, author_role, visibility, body, event, created_at
              FROM cp_ticket_messages
              WHERE ticket_id = $1`
	if !includeInternal {
		query += ` AND visibility <> 'internal'`
	}
	query += ` ORDER BY created_at ASC, id ASC`

	rows, err := r.Pool.Query(ctx, query, ticketID)
	if err != nil {
		return nil, fmt.Errorf("infra.MessageRepository.List: %w", err)
	}
	defer rows.Close()

	out := []domain.Message{}
	for rows.Next() {
		var (
			message    domain.Message
			role       string
			visibility string
		)
		if err := rows.Scan(&message.ID, &message.TicketID, &message.AuthorID,
			&role, &visibility, &message.Body, &message.Event, &message.CreatedAt); err != nil {
			return nil, fmt.Errorf("infra.MessageRepository.List: %w", err)
		}
		message.AuthorRole = domain.Actor(role)
		message.Visibility = domain.Visibility(visibility)
		out = append(out, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.MessageRepository.List: %w", err)
	}

	return out, nil
}
