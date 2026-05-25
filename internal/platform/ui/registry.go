package ui

import (
	"slices"
	"strings"

	"github.com/a-h/templ"
)

type Component struct {
	Render func() templ.Component
	Name   string
	Title  string
	Usage  string
}

var registry = map[string]Component{}

func Register(c Component) {
	if c.Name == "" {
		panic("ui: Component.Name required")
	}
	if c.Title == "" {
		panic("ui: Component.Title required")
	}
	if c.Render == nil {
		panic("ui: Component.Render required for " + c.Name)
	}
	if _, exists := registry[c.Name]; exists {
		panic("ui: duplicate component name: " + c.Name)
	}

	registry[c.Name] = c
}

func List() []Component {
	out := make([]Component, 0, len(registry))

	for _, c := range registry {
		out = append(out, c)
	}

	slices.SortFunc(out, func(a, b Component) int {
		return strings.Compare(a.Name, b.Name)
	})

	return out
}

func Get(name string) (Component, bool) {
	c, ok := registry[name]

	return c, ok
}
