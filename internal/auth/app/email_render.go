package app

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
