package themepage

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/a-h/templ"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var (
	DevMode  bool
	DiskRoot string
)

var runtimeFrontmatterRe = regexp.MustCompile(`(?s)\A---\r?\n.*?\r?\n---\r?\n`)

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
	src = runtimeFrontmatterRe.ReplaceAll(src, nil)

	var buf bytes.Buffer

	if err := markdown.Convert(src, &buf); err != nil {
		return "", fmt.Errorf("markdown convert: %w", err)
	}

	return template.HTML(buf.String()), nil //nolint:gosec // goldmark without WithUnsafe strips raw HTML
}

func RenderMarkdownPage(w http.ResponseWriter, r *http.Request, layout httpx.Layout, title string, src []byte) error {
	html, err := RenderMarkdown(src)
	if err != nil {
		return err
	}

	wrapped := `<article class="prose prose-zinc dark:prose-invert max-w-3xl mx-auto p-8">` + string(html) + `</article>`
	child := templ.Raw(wrapped)
	ctx := templ.WithChildren(r.Context(), child)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := httpx.ActiveBase(layout, title).Render(ctx, w); err != nil {
		return fmt.Errorf("render base: %w", err)
	}

	return nil
}

func RenderMarkdownPageFrom(w http.ResponseWriter, r *http.Request, layout httpx.Layout, title, embeddedPath string, prerendered template.HTML) error {
	if DevMode {
		path := filepath.Join(DiskRoot, embeddedPath)

		src, err := os.ReadFile(path) //nolint:gosec // themegen-controlled path
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		return RenderMarkdownPage(w, r, layout, title, src)
	}

	wrapped := `<article class="prose prose-zinc dark:prose-invert max-w-3xl mx-auto p-8">` + string(prerendered) + `</article>`
	child := templ.Raw(wrapped)
	ctx := templ.WithChildren(r.Context(), child)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := httpx.ActiveBase(layout, title).Render(ctx, w); err != nil {
		return fmt.Errorf("render base: %w", err)
	}

	return nil
}
