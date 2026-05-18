package httpx

import (
	"context"
	"strings"
	"testing"
)

func TestLocalTime_EmptyRendersNever(t *testing.T) {
	t.Parallel()

	var sb strings.Builder
	if err := LocalTime("").Render(context.Background(), &sb); err != nil {
		t.Fatalf("Render: %v", err)
	}
	body := sb.String()

	if !strings.Contains(body, "never") {
		t.Errorf("empty iso must render 'never'; body=%q", body)
	}
	if strings.Contains(body, "datetime=") {
		t.Errorf("empty iso must not emit a datetime attribute; body=%q", body)
	}
	if strings.Contains(body, "x-init") {
		t.Errorf("empty iso must not emit alpine init; body=%q", body)
	}
}

func TestLocalTime_PopulatedRendersDatetimeAndFallback(t *testing.T) {
	t.Parallel()

	iso := "2026-05-18T12:34:56Z"
	var sb strings.Builder
	if err := LocalTime(iso).Render(context.Background(), &sb); err != nil {
		t.Fatalf("Render: %v", err)
	}
	body := sb.String()

	if !strings.Contains(body, `datetime="`+iso+`"`) {
		t.Errorf("output must include datetime attribute with iso; body=%q", body)
	}
	if !strings.Contains(body, "x-init") {
		t.Errorf("output must include alpine x-init binding; body=%q", body)
	}
	if !strings.Contains(body, iso) {
		t.Errorf("output must include iso as no-JS fallback text; body=%q", body)
	}
	if strings.Contains(body, ">never<") {
		t.Errorf("populated iso must not render 'never' literal; body=%q", body)
	}
}
