package plugin

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
)

type Plugin struct {
	Mount      func(reg *routes.Registry, mux *http.ServeMux, in *infra.Infra)
	Middleware func(in *infra.Infra, h http.Handler) http.Handler
	Name       string
}

var (
	registry []Plugin
	mounted  bool
)

// Register adds p to the package plugin registry in registration order.
// It panics if MountAll has already been called, if p.Name is empty, if p.Mount is nil, or if a plugin with the same Name is already registered.
func Register(p Plugin) {
	if mounted {
		panic("plugin: Register called after MountAll: " + p.Name)
	}
	if p.Name == "" {
		panic("plugin: Name required")
	}
	if p.Mount == nil && p.Middleware == nil {
		panic("plugin: Mount or Middleware required for " + p.Name)
	}

	for _, existing := range registry {
		if existing.Name == p.Name {
			panic("plugin: duplicate plugin name: " + p.Name)
		}
	}

	registry = append(registry, p)
}

// MountAll mounts all registered plugins onto the provided HTTP ServeMux using the given Infra.
// Plugins are mounted in registration order, and each mount is logged via in.Logger.Info. MountAll panics if called more than once.
func MountAll(reg *routes.Registry, mux *http.ServeMux, in *infra.Infra) {
	if reg == nil {
		panic("plugin: MountAll requires non-nil routes registry")
	}
	if mux == nil {
		panic("plugin: MountAll requires non-nil mux")
	}
	if in == nil || in.Logger == nil {
		panic("plugin: MountAll requires non-nil infra with logger")
	}
	if mounted {
		panic("plugin: MountAll called more than once")
	}
	mounted = true

	for _, p := range registry {
		if p.Mount != nil {
			p.Mount(reg, mux, in)
		}
		in.Logger.Info("plugin mounted", "name", p.Name)
	}
}

// Middlewares returns registered plugins that supply a Middleware, in registration order.
func Middlewares() []Plugin {
	out := make([]Plugin, 0, len(registry))
	for _, p := range registry {
		if p.Middleware != nil {
			out = append(out, p)
		}
	}

	return out
}
