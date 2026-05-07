// Package plugin provides a lightweight plugin registry. Feature packages
// register themselves during init() via Register; the server then calls
// MountAll once to attach every plugin's routes to the HTTP mux.
package plugin

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/infra"
)

// Plugin describes a mountable feature module. Each plugin has a unique Name
// and a Mount function that wires the plugin's routes onto the provided mux
// using the shared infrastructure in in.
type Plugin struct {
	Mount func(mux *http.ServeMux, in *infra.Infra)
	Name  string
}

var (
	registry []Plugin
	mounted  bool
)

// Register adds p to the global plugin registry. It panics if:
//   - MountAll has already been called (late registration is not allowed),
//   - p.Name is empty,
//   - p.Mount is nil, or
//   - a plugin with the same name is already registered.
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

// MountAll calls each registered plugin's Mount function in registration order,
// passing mux and in. It sets a flag that prevents further calls to Register
// and logs each successful mount via in.Logger.
func MountAll(mux *http.ServeMux, in *infra.Infra) {
	mounted = true
	for _, p := range registry {
		p.Mount(mux, in)
		in.Logger.Info("plugin mounted", "name", p.Name)
	}
}
