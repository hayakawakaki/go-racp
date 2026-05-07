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

// Register registers the given Plugin so it will be mounted by MountAll.
// It validates the plugin and appends it to the internal registry.
// It panics if Register is called after MountAll, if the plugin Name is empty,
// if the plugin Mount callback is nil, or if a plugin with the same Name
// has already been registered.
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

// MountAll mounts all registered plugins onto mux using the provided infra.
// It panics if called more than once. Plugins are invoked in registration order;
// after each plugin is mounted a "plugin mounted" message with the plugin name
// is logged via in.Logger.
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
