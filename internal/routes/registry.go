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
	sessSvc      middleware.SessionValidator
	users        middleware.UserLookup
	cfg          config.RolesConfig
	logger       *slog.Logger
	registered   map[string]struct{}
	hiddenLayout httpx.Layout
	ungated      []ungatedRoute
	resolver     domain.RoleResolver
	secure       bool
}

type ungatedRoute struct {
	tag     string
	pattern string
}

func NewRegistry(
	cfg config.RolesConfig,
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

func (r *Registry) Wrap(mux *http.ServeMux, tag, pattern string, handler http.Handler) {
	group, action := parseTag(tag)
	r.registered[tag] = struct{}{}

	if group == adminGroup {
		mux.Handle(pattern, middleware.RequireRoleHidden(r.sessSvc, r.users, r.resolver, r.logger, r.secure, r.hiddenLayout)(handler))
		return
	}

	roles, configured := r.lookup(group, action)
	if !configured {
		r.ungated = append(r.ungated, ungatedRoute{tag: tag, pattern: pattern})
		mux.Handle(pattern, handler)
		return
	}

	mux.Handle(pattern, middleware.RequireRole(r.sessSvc, r.users, r.resolver, r.logger, r.secure, roles...)(handler))
}

func (r *Registry) lookup(group, action string) ([]domain.Role, bool) {
	actions, ok := r.cfg[group]
	if !ok {
		return nil, false
	}
	list, ok := actions[action]
	if !ok || len(list) == 0 {
		return nil, false
	}
	roles := make([]domain.Role, 0, len(list))
	for _, name := range list {
		roles = append(roles, mapRoleName(name))
	}

	return roles, true
}

func mapRoleName(name string) domain.Role {
	switch name {
	case "*":
		return domain.RoleAuthenticated
	case "Player":
		return domain.RolePlayer
	case "Event":
		return domain.RoleEvent
	case "Moderator":
		return domain.RoleModerator
	case "Enforcer":
		return domain.RoleEnforcer
	default:
		panic(fmt.Errorf("routes: unmapped role %q (validateRolesConfig should have caught this)", name))
	}
}

func parseTag(tag string) (group, action string) {
	parts := strings.Split(tag, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		panic(fmt.Errorf("routes: invalid tag %q — expected exactly Group.Action with non-empty segments", tag))
	}

	return parts[0], parts[1]
}

func (r *Registry) Finalize() error {
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
		panic(fmt.Errorf("roles.yml references tags not registered by any plugin: %s", strings.Join(deadEntries, ", ")))
	}

	for _, ungated := range r.ungated {
		r.logger.Warn("route audit: ungated", "tag", ungated.tag, "pattern", ungated.pattern)
	}

	return nil
}
