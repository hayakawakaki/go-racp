package routes

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/server/config"
)

const adminGroup = "Admin"

type Registry struct {
	hiddenLayout httpx.Layout
	sessSvc      middleware.SessionValidator
	users        middleware.UserLookup
	resolver     domain.RoleResolver
	cfg          config.AccessConfig
	logger       *slog.Logger
	registered   map[string]struct{}
	ungated      []ungatedRoute
	secure       bool
}

type ungatedRoute struct {
	tag     string
	pattern string
}

func NewRegistry(
	cfg config.AccessConfig,
	resolver domain.RoleResolver,
	sessSvc middleware.SessionValidator,
	users middleware.UserLookup,
	logger *slog.Logger,
	secure bool,
	hiddenLayout httpx.Layout,
) *Registry {
	return &Registry{
		cfg:          cfg,
		resolver:     resolver,
		sessSvc:      sessSvc,
		users:        users,
		logger:       logger,
		secure:       secure,
		hiddenLayout: hiddenLayout,
		registered:   make(map[string]struct{}),
	}
}

func (r *Registry) Public(mux *http.ServeMux, pattern string, handler http.Handler) {
	mux.Handle(pattern, handler)
}

// Wrap mounts handler under pattern with role gating derived from tag ("Group.Action"). Admin tags are hardcoded, other tags consult access.yml and pass through ungated (recorded for audit) when no entry is configured.
func (r *Registry) Wrap(mux *http.ServeMux, tag, pattern string, handler http.Handler) {
	group, action := parseTag(tag)
	r.registered[tag] = struct{}{}

	if group == adminGroup {
		mux.Handle(pattern, middleware.RequireRoleHidden(r.sessSvc, r.users, r.resolver, r.logger, r.secure, r.hiddenLayout, true)(handler))
		return
	}

	entry, configured := r.lookup(group, action)
	if !configured {
		r.ungated = append(r.ungated, ungatedRoute{tag: tag, pattern: pattern})
		mux.Handle(pattern, middleware.RequireRole(r.sessSvc, r.users, r.resolver, r.logger, r.secure, false, domain.RoleAuthenticated)(handler))
		return
	}

	mux.Handle(pattern, middleware.RequireRole(r.sessSvc, r.users, r.resolver, r.logger, r.secure, entry.unrestricted, entry.roles...)(handler))
}

type resolvedEntry struct {
	roles        []domain.Role
	unrestricted bool
}

func (r *Registry) lookup(group, action string) (resolvedEntry, bool) {
	actions, ok := r.cfg[group]
	if !ok {
		return resolvedEntry{}, false
	}
	cfgEntry, ok := actions[action]
	if !ok || len(cfgEntry.Roles) == 0 {
		return resolvedEntry{}, false
	}
	roles := make([]domain.Role, 0, len(cfgEntry.Roles))
	for _, name := range cfgEntry.Roles {
		role, ok := r.resolver.GetRole(name)
		if !ok {
			panic(fmt.Errorf("routes: access.yml entry %q.%q references unknown role %q. Add it under UserRoles in config.yml", group, action, name))
		}
		roles = append(roles, role)
	}

	return resolvedEntry{roles: roles, unrestricted: cfgEntry.RequiresUnrestricted()}, true
}

func parseTag(tag string) (group, action string) {
	parts := strings.Split(tag, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		panic(fmt.Errorf("routes: invalid tag %q, expected exactly Group.Action with non-empty segments", tag))
	}

	return parts[0], parts[1]
}

// Finalize panics if access.yml references tags no plugin registered, and logs a warning for every route that was mounted without an access.yml entry.
func (r *Registry) Finalize() {
	var deadEntries []string
	for groupName, actions := range r.cfg {
		for actionName := range actions {
			tag := groupName + "." + actionName
			if _, ok := r.registered[tag]; !ok {
				deadEntries = append(deadEntries, tag)
			}
		}
	}
	if len(deadEntries) > 0 {
		panic(fmt.Errorf("access.yml references tags not registered by any plugin: %s", strings.Join(deadEntries, ", ")))
	}

	for _, ungated := range r.ungated {
		r.logger.Warn("route audit: ungated", "tag", ungated.tag, "pattern", ungated.pattern)
	}
}
