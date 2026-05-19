package domain

type Category struct {
	Key     string
	Display string
	Roles   []string
}

type CategoryResolver struct {
	byKey map[string]Category
}

func NewCategoryResolver(categories []Category) CategoryResolver {
	byKey := make(map[string]Category, len(categories))
	for _, category := range categories {
		byKey[category.Key] = category
	}

	return CategoryResolver{byKey: byKey}
}

func (r CategoryResolver) Get(key string) (Category, bool) {
	category, ok := r.byKey[key]

	return category, ok
}

func (r CategoryResolver) All() []Category {
	out := make([]Category, 0, len(r.byKey))
	for _, category := range r.byKey {
		out = append(out, category)
	}

	return out
}

func (r CategoryResolver) AllowedForRole(roleName string, isAdmin bool) []string {
	out := make([]string, 0, len(r.byKey))
	for key, category := range r.byKey {
		if r.permits(category, roleName, isAdmin) {
			out = append(out, key)
		}
	}

	return out
}

func (r CategoryResolver) Permits(key, roleName string, isAdmin bool) bool {
	category, ok := r.byKey[key]
	if !ok {
		return isAdmin
	}

	return r.permits(category, roleName, isAdmin)
}

func (r CategoryResolver) permits(category Category, roleName string, isAdmin bool) bool {
	if isAdmin {
		return true
	}
	for _, role := range category.Roles {
		if role == "*" || role == roleName {
			return true
		}
	}

	return false
}
