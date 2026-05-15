package domain

type Category struct {
	Key     string
	Display string
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

func (r CategoryResolver) Has(key string) bool {
	_, ok := r.byKey[key]

	return ok
}

func (r CategoryResolver) All() []Category {
	out := make([]Category, 0, len(r.byKey))
	for _, category := range r.byKey {
		out = append(out, category)
	}

	return out
}

func (r CategoryResolver) Display(key string) string {
	if category, ok := r.byKey[key]; ok {
		return category.Display
	}

	return key
}
