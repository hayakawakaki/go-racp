package self

import (
	"bytes"
	"context"
	"fmt"

	mailtemplate "github.com/hayakawakaki/go-racp/internal/infra/mailer/template"
)

func renderVerificationEmail(ctx context.Context, d mailtemplate.VerificationData) (string, error) {
	var buf bytes.Buffer
	if err := mailtemplate.Verification(d).Render(ctx, &buf); err != nil {
		return "", fmt.Errorf("renderVerificationEmail: %w", err)
	}

	return buf.String(), nil
}

func renderPasswordResetEmail(ctx context.Context, d mailtemplate.PasswordResetData) (string, error) {
	var buf bytes.Buffer
	if err := mailtemplate.PasswordReset(d).Render(ctx, &buf); err != nil {
		return "", fmt.Errorf("renderPasswordResetEmail: %w", err)
	}

	return buf.String(), nil
}

func renderEmailChangeEmail(ctx context.Context, d mailtemplate.EmailChangeData) (string, error) {
	var buf bytes.Buffer
	if err := mailtemplate.EmailChange(d).Render(ctx, &buf); err != nil {
		return "", fmt.Errorf("renderEmailChangeEmail: %w", err)
	}

	return buf.String(), nil
}
