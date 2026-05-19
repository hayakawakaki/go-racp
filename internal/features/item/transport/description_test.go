package transport

import (
	"strings"
	"testing"
)

func TestRenderDescription_PlainText(t *testing.T) {
	got := string(renderDescription([]string{"A simple description."}))
	if got != "A simple description." {
		t.Errorf("got %q, want %q", got, "A simple description.")
	}
}

func TestRenderDescription_EscapesHTML(t *testing.T) {
	got := string(renderDescription([]string{"<script>alert(1)</script>"}))
	if strings.Contains(got, "<script>") {
		t.Errorf("got %q, raw <script> tag should have been escaped", got)
	}
}

func TestRenderDescription_ColorSpan(t *testing.T) {
	got := string(renderDescription([]string{"^FF0000Red text^000000 normal"}))
	want := `<span style="color: #FF0000">Red text</span> normal`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderDescription_MultipleColorSpans(t *testing.T) {
	got := string(renderDescription([]string{"^FF0000A^000000 ^00FF00B^000000"}))
	want := `<span style="color: #FF0000">A</span> <span style="color: #00FF00">B</span>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderDescription_TIPBOX(t *testing.T) {
	got := string(renderDescription([]string{"<TIPBOX>Tip<INFO>body</INFO></TIPBOX>"}))
	want := `<span class="text-blue-400">Tip</span>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderDescription_NAVI(t *testing.T) {
	line := "<NAVI>Prontera Square<INFO>prontera,156,191,0,0,0,0,0</INFO></NAVI>"
	got := string(renderDescription([]string{line}))
	if !strings.Contains(got, `data-navi="/navi prontera 156/191"`) {
		t.Errorf("got %q, missing data-navi payload", got)
	}
	if !strings.Contains(got, "Prontera Square") {
		t.Errorf("got %q, missing label", got)
	}
}

func TestRenderDescription_MalformedColorDegrades(t *testing.T) {
	got := string(renderDescription([]string{"^GG0000Bad color"}))
	if strings.Contains(got, "<span") {
		t.Errorf("got %q, malformed color should not produce a span", got)
	}
}

func TestRenderDescription_MultiLineJoined(t *testing.T) {
	got := string(renderDescription([]string{"line 1", "line 2"}))
	if !strings.Contains(got, "line 1") || !strings.Contains(got, "line 2") {
		t.Errorf("got %q, missing one of the lines", got)
	}
}
