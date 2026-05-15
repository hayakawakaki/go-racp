package infra

import (
	"bytes"
	"html/template"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

type Renderer struct {
	md     goldmark.Markdown
	policy *bluemonday.Policy
}

func NewRenderer() *Renderer {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithHardWraps()),
	)

	return &Renderer{md: md, policy: bluemonday.UGCPolicy()}
}

func (r *Renderer) Render(markdown string) template.HTML {
	var buf bytes.Buffer
	if err := r.md.Convert([]byte(markdown), &buf); err != nil {
		return ""
	}

	return template.HTML(r.policy.SanitizeBytes(buf.Bytes())) //nolint:gosec // bluemonday strips dangerous content.
}
