package routes

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/security"
	"github.com/hayakawakaki/go-racp/server/config"
)

const adminGroup = "Admin"

type Registry struct {
	hiddenLayout         httpx.Layout
	sessSvc              middleware.SessionValidator
	users                middleware.UserLookup
	resolver             domain.RoleResolver
	cfg                  config.AccessConfig
	limiters             map[string]*security.RateLimiter
	logger               *slog.Logger
	registered           map[string]struct{}
	ungated              []ungatedRoute
	secure               bool
	allowTempBannedLogin bool
}

type ungatedRoute struct {
	tag     string
	pattern string
}

func NewRegistry(
	cfg config.AccessConfig,
	limiters map[string]*security.RateLimiter,
	resolver domain.RoleResolver,
	sessSvc middleware.SessionValidator,
	users middleware.UserLookup,
	logger *slog.Logger,
	secure bool,
	allowTempBannedLogin bool,
	hiddenLayout httpx.Layout,
) *Registry {
	return &Registry{
		cfg:                  cfg,
		limiters:             limiters,
		resolver:             resolver,
		sessSvc:              sessSvc,
		users:                users,
		logger:               logger,
		secure:               secure,
		allowTempBannedLogin: allowTempBannedLogin,
		hiddenLayout:         hiddenLayout,
		registered:           make(map[string]struct{}),
	}
}

// Wrap mounts handler under pattern with role gating derived from tag ("Group.Action"). Admin tags are hardcoded, other tags consult access.yml and pass through ungated (recorded for audit) when no entry is configured.
func (r *Registry) Wrap(mux *http.ServeMux, tag, pattern string, handler http.Handler) {
	group, action := parseTag(tag)
	r.registered[tag] = struct{}{}

	if limiter, ok := r.limiters[tag]; ok {
		handler = limiter.Middleware(handler)
	}

	if group == adminGroup {
		r.mountAdminOnly(mux, pattern, handler)
		return
	}

	entry, configured := r.lookup(group, action)
	if !configured {
		r.ungated = append(r.ungated, ungatedRoute{tag: tag, pattern: pattern})
		policy := middleware.AuthPolicy{AllowTempBannedLogin: r.allowTempBannedLogin}
		mux.Handle(pattern, middleware.RequireRole(r.sessSvc, r.users, r.resolver, r.logger, r.secure, policy, domain.RoleAuthenticated)(handler))
		return
	}

	if len(entry.roles) == 1 && entry.roles[0] == domain.RoleAdmin {
		r.mountAdminOnly(mux, pattern, handler)
		return
	}

	if len(entry.roles) == 1 && entry.roles[0] == domain.RolePublic {
		mux.Handle(pattern, handler)
		return
	}

	policy := middleware.AuthPolicy{AllowTempBannedLogin: r.allowTempBannedLogin, Unrestricted: entry.unrestricted}
	mux.Handle(pattern, middleware.RequireRole(r.sessSvc, r.users, r.resolver, r.logger, r.secure, policy, entry.roles...)(handler))
}

func (r *Registry) mountAdminOnly(mux *http.ServeMux, pattern string, handler http.Handler) {
	policy := middleware.AuthPolicy{AllowTempBannedLogin: r.allowTempBannedLogin, Unrestricted: true}
	mux.Handle(pattern, middleware.RequireRoleHidden(r.sessSvc, r.users, r.resolver, r.logger, r.secure, r.hiddenLayout, policy)(handler))
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
	if !ok {
		return resolvedEntry{}, false
	}
	if len(cfgEntry.Roles) == 0 {
		panic(fmt.Errorf("routes: access.yml entry %q.%q must declare at least one role", group, action))
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

	if len(r.ungated) > 0 {
		descriptions := make([]string, 0, len(r.ungated))
		for _, u := range r.ungated {
			descriptions = append(descriptions, u.tag+" ("+u.pattern+")")
		}
		panic(fmt.Errorf("routes: tags registered without access.yml entries: %s", strings.Join(descriptions, ", ")))
	}
}
