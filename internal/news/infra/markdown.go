package infra

import (
	"bytes"
	"html/template"
	"log/slog"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

type Renderer struct {
	md     goldmark.Markdown
	policy *bluemonday.Policy
	logger *slog.Logger
}

func NewRenderer(logger *slog.Logger) *Renderer {
	if logger == nil {
		logger = slog.Default()
	}
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithHardWraps()),
	)

	return &Renderer{md: md, policy: bluemonday.UGCPolicy(), logger: logger}
}

func (r *Renderer) Render(markdown string) template.HTML {
	var buf bytes.Buffer
	if err := r.md.Convert([]byte(markdown), &buf); err != nil {
		r.logger.Warn("news: markdown render failed", "err", err)
		return ""
	}

	return template.HTML(r.policy.SanitizeBytes(buf.Bytes())) //nolint:gosec // bluemonday strips dangerous content.
}
