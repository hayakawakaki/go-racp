package mailer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	gomail "github.com/wneessen/go-mail"
)

const sendTimeout = 30 * time.Second

type SMTPMailer struct {
	client *gomail.Client
	logger *slog.Logger
	from   string
}

func NewSMTPMailer(client *gomail.Client, from string, logger *slog.Logger) *SMTPMailer {
	return &SMTPMailer{
		client: client,
		from:   from,
		logger: logger,
	}
}

func (m *SMTPMailer) Close() error {
	if err := m.client.Close(); err != nil {
		return fmt.Errorf("mailer.SMTPMailer.Close: %w", err)
	}

	return nil
}

func (m *SMTPMailer) Send(ctx context.Context, to, subject, body string) error {
	msg := gomail.NewMsg()
	if err := msg.From(m.from); err != nil {
		return fmt.Errorf("mailer.SMTPMailer.Send: from: %w", err)
	}
	if err := msg.To(to); err != nil {
		return fmt.Errorf("mailer.SMTPMailer.Send: to: %w", err)
	}
	msg.Subject(subject)
	msg.SetBodyString(gomail.TypeTextHTML, body)

	if err := m.client.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("mailer.SMTPMailer.Send: %w", err)
	}

	return nil
}

// SendAsync dispatches a message in a background goroutine with a bounded timeout and logs any error.
func (m *SMTPMailer) SendAsync(to, subject, body string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
		defer cancel()
		if err := m.Send(ctx, to, subject, body); err != nil {
			m.logger.Error("mail send failed", "to", to, "subject", subject, "err", err)
		}
	}()
}
