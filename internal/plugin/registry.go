package plugin

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/infra"
)

type Plugin struct {
	Mount func(mux *http.ServeMux, in *infra.Infra)
	Name  string
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
	if p.Mount == nil {
		panic("plugin: Mount required for " + p.Name)
	}
	for _, existing := range registry {
		if existing.Name == p.Name {
			panic("plugin: duplicate plugin name: " + p.Name)
		}
	}
	registry = append(registry, p)
}

// MountAll mounts all registered plugins onto the provided HTTP ServeMux using the given Infra.
// Plugins are mounted in registration order; each mount is logged via in.Logger.Info. MountAll panics if called more than once.
func MountAll(mux *http.ServeMux, in *infra.Infra) {
	if mounted {
		panic("plugin: MountAll called more than once")
	}
	mounted = true
	for _, p := range registry {
		p.Mount(mux, in)
		in.Logger.Info("plugin mounted", "name", p.Name)
	}
}
