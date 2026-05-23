package themepage

import (
	"strings"
	"testing"
)

func TestRenderMarkdown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name:     "plain paragraph",
			input:    "hello world",
			contains: []string{"<p>hello world</p>"},
		},
		{
			name:     "h1 heading",
			input:    "# Title",
			contains: []string{"<h1>Title</h1>"},
		},
		{
			name:     "h2 heading",
			input:    "## Subtitle",
			contains: []string{"<h2>Subtitle</h2>"},
		},
		{
			name:     "bold",
			input:    "**important**",
			contains: []string{"<strong>important</strong>"},
		},
		{
			name:     "italic",
			input:    "*emphasized*",
			contains: []string{"<em>emphasized</em>"},
		},
		{
			name:     "inline code",
			input:    "use `@warp` in chat",
			contains: []string{"<code>@warp</code>"},
		},
		{
			name:     "code block",
			input:    "```\nlet x = 1\n```",
			contains: []string{"<pre>", "<code>", "let x = 1"},
		},
		{
			name:     "unordered list",
			input:    "- one\n- two\n- three",
			contains: []string{"<ul>", "<li>one</li>", "<li>two</li>", "<li>three</li>"},
		},
		{
			name:     "ordered list",
			input:    "1. first\n2. second",
			contains: []string{"<ol>", "<li>first</li>", "<li>second</li>"},
		},
		{
			name:     "blockquote",
			input:    "> quoted line",
			contains: []string{"<blockquote>", "quoted line"},
		},
		{
			name:     "horizontal rule",
			input:    "before\n\n---\n\nafter",
			contains: []string{"<hr>"},
		},
		{
			name:     "explicit link",
			input:    "[example](https://example.com)",
			contains: []string{`<a href="https://example.com">example</a>`},
		},
		{
			name:     "GFM autolink URL",
			input:    "visit https://example.com today",
			contains: []string{`<a href="https://example.com">https://example.com</a>`},
		},
		{
			name:     "GFM autolink email",
			input:    "contact admin@example.com please",
			contains: []string{`<a href="mailto:admin@example.com">admin@example.com</a>`},
		},
		{
			name:     "GFM strikethrough",
			input:    "~~deleted~~ kept",
			contains: []string{"<del>deleted</del>"},
		},
		{
			name:  "GFM table",
			input: "| A | B |\n|---|---|\n| 1 | 2 |\n| 3 | 4 |",
			contains: []string{
				"<table>",
				"<thead>",
				"<tbody>",
				"<th>A</th>",
				"<th>B</th>",
				"<td>1</td>",
				"<td>2</td>",
				"<td>3</td>",
				"<td>4</td>",
			},
		},
		{
			name:  "GFM task list",
			input: "- [x] done\n- [ ] todo",
			contains: []string{
				`<input checked="" disabled="" type="checkbox">`,
				`<input disabled="" type="checkbox">`,
				"done",
				"todo",
			},
		},
		{
			name:     "empty input",
			input:    "",
			contains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := RenderMarkdown([]byte(tt.input))
			if err != nil {
				t.Fatalf("RenderMarkdown returned error: %v", err)
			}

			html := string(got)
			for _, want := range tt.contains {
				if !strings.Contains(html, want) {
					t.Errorf("output missing %q\nfull output:\n%s", want, html)
				}
			}
		})
	}
}

func TestRenderMarkdown_HTMLEscaping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		mustNot  []string
		mustHave []string
	}{
		{
			name:     "inline code escapes angle brackets",
			input:    "use `<script>` carefully",
			mustNot:  []string{"<script>"},
			mustHave: []string{"&lt;script&gt;"},
		},
		{
			name:     "code block escapes content",
			input:    "```\n<script>alert(1)</script>\n```",
			mustNot:  []string{"<script>alert(1)</script>"},
			mustHave: []string{"&lt;script&gt;", "&lt;/script&gt;"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := RenderMarkdown([]byte(tt.input))
			if err != nil {
				t.Fatalf("RenderMarkdown returned error: %v", err)
			}

			html := string(got)
			for _, bad := range tt.mustNot {
				if strings.Contains(html, bad) {
					t.Errorf("output contains forbidden %q\nfull output:\n%s", bad, html)
				}
			}

			for _, want := range tt.mustHave {
				if !strings.Contains(html, want) {
					t.Errorf("output missing required %q\nfull output:\n%s", want, html)
				}
			}
		})
	}
}
