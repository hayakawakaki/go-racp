package app

import (
	"bytes"
	"context"
	"fmt"

	"github.com/hayakawakaki/go-racp/internal/features/tickets/domain"
	"github.com/hayakawakaki/go-racp/internal/features/tickets/notifications"
)

func (s *Service) notifyStaffReply(ctx context.Context, ticket domain.Ticket) {
	s.sendEmail(ctx, ticket, "Staff replied to your ticket.")
}

func (s *Service) notifyTerminal(ctx context.Context, ticket domain.Ticket) {
	label := "Your ticket was resolved."
	if ticket.Status == domain.StatusClosed {
		label = "Your ticket was closed."
	}
	s.sendEmail(ctx, ticket, label)
}

func (s *Service) sendEmail(ctx context.Context, ticket domain.Ticket, eventLabel string) {
	if s.mailer == nil {
		return
	}
	user, err := s.users.GetByID(ctx, ticket.AccountID)
	if err != nil {
		s.logger.Error("tickets.notify: user lookup", "err", err, "accountID", ticket.AccountID)
		return
	}
	if user.Email == "" {
		return
	}

	body, err := renderEmail(ctx, notifications.EmailData{
		ServerName: s.config.ServerName,
		Subject:    ticket.Subject,
		TicketNo:   ticket.ID,
		EventLabel: eventLabel,
		URL:        fmt.Sprintf("%s/tickets/%d", s.config.AppURL, ticket.ID),
	})
	if err != nil {
		s.logger.Error("tickets.notify: render", "err", err)
		return
	}

	subject := fmt.Sprintf("[%s] Ticket #%d", s.config.ServerName, ticket.ID)
	s.mailer.SendAsync(user.Email, subject, body)
}

func renderEmail(ctx context.Context, data notifications.EmailData) (string, error) {
	var buf bytes.Buffer
	if err := notifications.Email(data).Render(ctx, &buf); err != nil {
		return "", fmt.Errorf("app.renderEmail: %w", err)
	}

	return buf.String(), nil
}
