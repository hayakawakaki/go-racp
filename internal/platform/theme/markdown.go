package theme

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"

	"github.com/a-h/templ"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var markdown = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		extension.Strikethrough,
		extension.Table,
		extension.TaskList,
		extension.Linkify,
	),
)

func RenderMarkdown(src []byte) (template.HTML, error) {
	var buf bytes.Buffer

	if err := markdown.Convert(src, &buf); err != nil {
		return "", fmt.Errorf("markdown convert: %w", err)
	}

	return template.HTML(buf.String()), nil //nolint:gosec // markdown rendered with safe defaults
}

func RenderMarkdownPage(w http.ResponseWriter, r *http.Request, layout httpx.Layout, title string, src []byte) error {
	html, err := RenderMarkdown(src)
	if err != nil {
		return err
	}

	child := templ.Raw(string(html))
	ctx := templ.WithChildren(r.Context(), child)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := httpx.ActiveBase(layout, title).Render(ctx, w); err != nil {
		return fmt.Errorf("render base: %w", err)
	}

	return nil
}
