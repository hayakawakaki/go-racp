package app

import (
	"bytes"
	"context"
	"fmt"

	mailtemplate "github.com/hayakawakaki/go-racp/internal/infra/mailer/template"
)

func renderEmailChangeEmail(ctx context.Context, d mailtemplate.EmailChangeData) (string, error) {
	var buf bytes.Buffer
	if err := mailtemplate.EmailChange(d).Render(ctx, &buf); err != nil {
		return "", fmt.Errorf("renderEmailChangeEmail: %w", err)
	}
	return buf.String(), nil
}
