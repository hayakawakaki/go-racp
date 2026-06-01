package domain

import "testing"

func newTestTierSet() TierSet {
	return NewTierSet([]Tier{
		{Name: "Standard", RatePerMinute: 180, Burst: 180},
		{Name: "Elevated", RatePerMinute: 600, Burst: 600},
	})
}

func TestTierSet_Has(t *testing.T) {
	t.Parallel()

	tiers := newTestTierSet()

	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{name: "known standard", query: "Standard", want: true},
		{name: "known elevated", query: "Elevated", want: true},
		{name: "unknown tier", query: "Platinum", want: false},
		{name: "empty query", query: "", want: false},
		{name: "case sensitive", query: "standard", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tiers.Has(tt.query); got != tt.want {
				t.Errorf("Has(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestTierSet_Get(t *testing.T) {
	t.Parallel()

	tiers := newTestTierSet()

	tests := []struct {
		name   string
		query  string
		want   Tier
		wantOk bool
	}{
		{name: "known elevated", query: "Elevated", want: Tier{Name: "Elevated", RatePerMinute: 600, Burst: 600}, wantOk: true},
		{name: "known standard", query: "Standard", want: Tier{Name: "Standard", RatePerMinute: 180, Burst: 180}, wantOk: true},
		{name: "unknown tier", query: "Platinum", want: Tier{}, wantOk: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := tiers.Get(tt.query)
			if ok != tt.wantOk {
				t.Errorf("Get(%q) ok = %v, want %v", tt.query, ok, tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("Get(%q) = %+v, want %+v", tt.query, got, tt.want)
			}
		})
	}
}

func TestTierSet_List_SortedByName(t *testing.T) {
	t.Parallel()

	tiers := newTestTierSet()

	got := tiers.List()
	if len(got) != 2 {
		t.Fatalf("List() len = %d, want 2", len(got))
	}
	if got[0].Name != "Elevated" || got[1].Name != "Standard" {
		t.Errorf("List() order = [%s %s], want [Elevated Standard]", got[0].Name, got[1].Name)
	}
}

func TestTierSet_List_Empty(t *testing.T) {
	t.Parallel()

	got := NewTierSet(nil).List()
	if len(got) != 0 {
		t.Errorf("List() len = %d, want 0", len(got))
	}
}
