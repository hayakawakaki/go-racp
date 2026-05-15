package news

import (
	"slices"
	"testing"

	"github.com/hayakawakaki/go-racp/server/config"
)

func TestBuildCategoryResolver_SortsKeysAlphabetically(t *testing.T) {
	t.Parallel()
	cfg := config.NewsCategoriesConfig{
		"Banana":       {Display: "Banana"},
		"Announcement": {Display: "Announcement"},
		"Patch":        {Display: "Patch Notes"},
	}

	resolver := buildCategoryResolver(cfg)
	got := resolver.All()
	gotKeys := make([]string, len(got))
	for i, c := range got {
		gotKeys[i] = c.Key
	}
	want := []string{"Announcement", "Patch", "Banana"}
	if !slices.Equal(gotKeys, want) {
		t.Errorf("keys = %v, want %v", gotKeys, want)
	}
}

func TestBuildCategoryResolver_EmptyConfig(t *testing.T) {
	t.Parallel()
	resolver := buildCategoryResolver(config.NewsCategoriesConfig{})
	if got := resolver.All(); len(got) != 0 {
		t.Errorf("All() = %v, want empty", got)
	}
}
