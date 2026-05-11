package mailer

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	gomail "github.com/wneessen/go-mail"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestClient(t *testing.T, host string, port int) *gomail.Client {
	t.Helper()
	client, err := NewClient(host, port)
	if err != nil {
		t.Fatalf("NewClient(%q, %d): %v", host, port, err)
	}
	return client
}

func TestNewSMTPMailer_FieldsWired(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, "smtp.example.com", 587)
	logger := discardLogger()

	mailer := NewSMTPMailer(client, "noreply@example.invalid", logger)

	if mailer == nil {
		t.Fatal("NewSMTPMailer returned nil")
	}
	if mailer.client != client {
		t.Errorf("client not wired")
	}
	if mailer.from != "noreply@example.invalid" {
		t.Errorf("from = %q, want noreply@example.invalid", mailer.from)
	}
	if mailer.logger != logger {
		t.Errorf("logger not wired")
	}
}

func TestSMTPMailer_Close_Idempotent(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, "smtp.example.com", 587)
	mailer := NewSMTPMailer(client, "noreply@example.invalid", discardLogger())

	if err := mailer.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := mailer.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestSMTPMailer_Send_InvalidFromAddress(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, "smtp.example.com", 587)
	mailer := NewSMTPMailer(client, "not a valid address", discardLogger())

	err := mailer.Send(context.Background(), "to@example.invalid", "subject", "body")
	if err == nil {
		t.Fatal("expected error for invalid from address")
	}
	if !strings.Contains(err.Error(), "mailer.SMTPMailer.Send") {
		t.Errorf("error not wrapped with prefix: %v", err)
	}
	if !strings.Contains(err.Error(), "from") {
		t.Errorf("error missing 'from' context: %v", err)
	}
}

func TestSMTPMailer_Send_InvalidToAddress(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, "smtp.example.com", 587)
	mailer := NewSMTPMailer(client, "noreply@example.invalid", discardLogger())

	err := mailer.Send(context.Background(), "not a valid recipient", "subject", "body")
	if err == nil {
		t.Fatal("expected error for invalid to address")
	}
	if !strings.Contains(err.Error(), "mailer.SMTPMailer.Send") {
		t.Errorf("error not wrapped with prefix: %v", err)
	}
	if !strings.Contains(err.Error(), "to") {
		t.Errorf("error missing 'to' context: %v", err)
	}
}

func TestSMTPMailer_Send_DialFailureWraps(t *testing.T) {
	t.Parallel()
	port := pickClosedPort(t)
	client := newTestClient(t, "127.0.0.1", port)
	mailer := NewSMTPMailer(client, "noreply@example.invalid", discardLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := mailer.Send(ctx, "to@example.invalid", "subject", "body")
	if err == nil {
		t.Fatal("expected dial error against closed port")
	}
	if !strings.Contains(err.Error(), "mailer.SMTPMailer.Send") {
		t.Errorf("error not wrapped: %v", err)
	}
}

func TestSMTPMailer_SendAsync_LogsOnFailure(t *testing.T) {
	t.Parallel()
	port := pickClosedPort(t)
	logBuffer := newSyncBuffer()
	logger := slog.New(slog.NewTextHandler(logBuffer, nil))
	client := newTestClient(t, "127.0.0.1", port)
	mailer := NewSMTPMailer(client, "noreply@example.invalid", logger)

	mailer.SendAsync("to@example.invalid", "subject", "body")

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(logBuffer.String(), "mail send failed") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("SendAsync goroutine never logged failure within deadline; log so far: %q", logBuffer.String())
}

func TestSMTPMailer_SendAsync_ReturnsImmediately(t *testing.T) {
	t.Parallel()
	port := pickClosedPort(t)
	client := newTestClient(t, "127.0.0.1", port)
	mailer := NewSMTPMailer(client, "noreply@example.invalid", discardLogger())

	start := time.Now()
	mailer.SendAsync("to@example.invalid", "subject", "body")
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Errorf("SendAsync blocked for %v, want fire-and-forget", elapsed)
	}
}

func pickClosedPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port
}

type syncBuffer struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func newSyncBuffer() *syncBuffer { return &syncBuffer{} }

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
