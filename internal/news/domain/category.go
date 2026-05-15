package domain

type Category struct {
	Key     string
	Display string
}

type CategoryResolver struct {
	byKey map[string]Category
	all   []Category
}

func NewCategoryResolver(categories []Category) CategoryResolver {
	byKey := make(map[string]Category, len(categories))
	for _, category := range categories {
		byKey[category.Key] = category
	}

	return CategoryResolver{byKey: byKey, all: categories}
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
	return r.all
}

func (r CategoryResolver) Display(key string) string {
	if category, ok := r.byKey[key]; ok {
		return category.Display
	}

	return key
}
