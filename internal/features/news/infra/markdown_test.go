package infra

import (
	"io"
	"log/slog"
	"strings"
	"testing"
)

func newTestRenderer() *Renderer {
	return NewRenderer(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestNewRenderer_NilLoggerUsesDefault(t *testing.T) {
	t.Parallel()
	r := NewRenderer(nil)
	if r == nil || r.logger == nil {
		t.Fatal("expected non-nil renderer and logger")
	}
}

func TestRenderer_Render_MarkdownFeatures(t *testing.T) {
	t.Parallel()
	r := newTestRenderer()

	tests := []struct {
		name       string
		input      string
		wantSubstr []string
	}{
		{
			name:       "heading",
			input:      "## Hello",
			wantSubstr: []string{"<h2", "Hello", "</h2>"},
		},
		{
			name:       "bold and italic",
			input:      "This is **bold** and *italic*",
			wantSubstr: []string{"<strong>bold</strong>", "<em>italic</em>"},
		},
		{
			name:       "fenced code",
			input:      "```\nfoo bar\n```",
			wantSubstr: []string{"<pre>", "<code>", "foo bar"},
		},
		{
			name:       "http link",
			input:      "[example](https://example.com)",
			wantSubstr: []string{`href="https://example.com"`, "example</a>"},
		},
		{
			name:       "GFM table",
			input:      "| H1 | H2 |\n|----|----|\n| a  | b  |",
			wantSubstr: []string{"<table>", "<thead>", "<tbody>", "<th>H1</th>", "<td>a</td>"},
		},
		{
			name:       "strikethrough",
			input:      "~~gone~~",
			wantSubstr: []string{"<del>gone</del>"},
		},
		{
			name:       "unordered list",
			input:      "- one\n- two",
			wantSubstr: []string{"<ul>", "<li>one</li>", "<li>two</li>"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := string(r.Render(tt.input))
			for _, want := range tt.wantSubstr {
				if !strings.Contains(got, want) {
					t.Errorf("Render(%q) missing %q\noutput: %s", tt.input, want, got)
				}
			}
		})
	}
}

func TestRenderer_Render_SanitizesDangerousContent(t *testing.T) {
	t.Parallel()
	r := newTestRenderer()

	tests := []struct {
		name        string
		input       string
		wantMissing []string
	}{
		{
			name:        "script tag in markdown is escaped not rendered",
			input:       "Hello <script>alert(1)</script>",
			wantMissing: []string{"<script>", "</script>"},
		},
		{
			name:        "javascript URL in link is stripped",
			input:       "[bad](javascript:alert(1))",
			wantMissing: []string{"javascript:"},
		},
		{
			name:        "data URL in image is stripped",
			input:       "![x](data:text/html,<script>alert(1)</script>)",
			wantMissing: []string{"data:text/html"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := string(r.Render(tt.input))
			for _, bad := range tt.wantMissing {
				if strings.Contains(got, bad) {
					t.Errorf("Render(%q) should not contain %q\noutput: %s", tt.input, bad, got)
				}
			}
		})
	}
}

func TestRenderer_Render_EmptyInput(t *testing.T) {
	t.Parallel()
	r := newTestRenderer()
	if got := r.Render(""); string(got) != "" {
		t.Errorf("Render(empty) = %q, want empty", got)
	}
}
