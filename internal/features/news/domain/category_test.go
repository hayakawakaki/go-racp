package domain

import (
	"slices"
	"testing"
)

func TestNewCategoryResolver_PreservesOrder(t *testing.T) {
	t.Parallel()
	input := []Category{
		{Key: "Announcement", Display: "Announcement"},
		{Key: "Update", Display: "Update"},
		{Key: "Patch", Display: "Patch Notes"},
	}
	resolver := NewCategoryResolver(input)

	got := resolver.All()
	if !slices.Equal(got, input) {
		t.Errorf("All() = %v, want %v", got, input)
	}
}

func TestCategoryResolver_Get(t *testing.T) {
	t.Parallel()
	resolver := NewCategoryResolver([]Category{
		{Key: "Patch", Display: "Patch Notes"},
	})

	if got, ok := resolver.Get("Patch"); !ok || got.Display != "Patch Notes" {
		t.Errorf("Get(Patch) = (%+v, %v); want (Patch Notes, true)", got, ok)
	}

	if _, ok := resolver.Get("Missing"); ok {
		t.Errorf("Get(Missing) ok = true, want false")
	}
}

func TestCategoryResolver_Has(t *testing.T) {
	t.Parallel()
	resolver := NewCategoryResolver([]Category{{Key: "Patch", Display: "Patch Notes"}})

	if !resolver.Has("Patch") {
		t.Errorf("Has(Patch) = false, want true")
	}
	if resolver.Has("Missing") {
		t.Errorf("Has(Missing) = true, want false")
	}
}

func TestCategoryResolver_Display(t *testing.T) {
	t.Parallel()
	resolver := NewCategoryResolver([]Category{
		{Key: "Patch", Display: "Patch Notes"},
	})

	if got := resolver.Display("Patch"); got != "Patch Notes" {
		t.Errorf("Display(Patch) = %q, want Patch Notes", got)
	}
	if got := resolver.Display("Unknown"); got != "Unknown" {
		t.Errorf("Display(Unknown) = %q, want fallback to key", got)
	}
}

func TestCategoryResolver_All_EmptyResolver(t *testing.T) {
	t.Parallel()
	resolver := NewCategoryResolver(nil)
	if got := resolver.All(); got != nil {
		t.Errorf("All() on empty resolver = %v, want nil", got)
	}
}
