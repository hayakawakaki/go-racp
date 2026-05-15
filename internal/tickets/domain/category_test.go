package domain

import "testing"

func newResolver() CategoryResolver {
	return NewCategoryResolver([]Category{
		{Key: "BugReport", Display: "Bug Report", Roles: []string{"Moderator", "Enforcer"}},
		{Key: "Donation", Display: "Donation", Roles: []string{"Moderator"}},
		{Key: "Other", Display: "Other", Roles: []string{"*"}},
	})
}

func TestCategoryResolver_Permits(t *testing.T) {
	t.Parallel()

	resolver := newResolver()
	tests := []struct {
		name    string
		key     string
		role    string
		isAdmin bool
		want    bool
	}{
		{"admin always", "BugReport", "Player", true, true},
		{"moderator on bug", "BugReport", "Moderator", false, true},
		{"enforcer on donation", "Donation", "Enforcer", false, false},
		{"wildcard other", "Other", "Enforcer", false, true},
		{"unknown key non-admin", "Missing", "Moderator", false, false},
		{"unknown key admin", "Missing", "Moderator", true, true},
	}
	for _, tt := range tests {
		if got := resolver.Permits(tt.key, tt.role, tt.isAdmin); got != tt.want {
			t.Errorf("%s: Permits() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestCategoryResolver_AllowedForRole(t *testing.T) {
	t.Parallel()

	resolver := newResolver()
	got := resolver.AllowedForRole("Moderator", false)
	if len(got) != 3 {
		t.Errorf("Moderator allowed = %v, want 3 categories", got)
	}

	got = resolver.AllowedForRole("Enforcer", false)
	if len(got) != 2 {
		t.Errorf("Enforcer allowed = %v, want 2 categories", got)
	}
}
