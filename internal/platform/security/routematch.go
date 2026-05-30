package security

import (
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strings"
)

type RouteMatcher struct {
	exact    map[string]struct{}
	patterns []*regexp.Regexp
}

func NewRouteMatcher(routes []string) (*RouteMatcher, error) {
	m := &RouteMatcher{exact: make(map[string]struct{}, len(routes))}
	for _, route := range routes {
		if !strings.HasPrefix(route, "/") {
			return nil, fmt.Errorf("security.NewRouteMatcher: route %q must start with /", route)
		}

		parts := strings.Split(route, "*")
		if len(parts) == 1 {
			m.exact[path.Clean(route)] = struct{}{}
			continue
		}

		if parts[0] == "/" {
			return nil, fmt.Errorf("security.NewRouteMatcher: route %q is too broad", route)
		}

		for i := 1; i < len(parts); i++ {
			if !strings.HasSuffix(parts[i-1], "/") {
				return nil, fmt.Errorf("security.NewRouteMatcher: route %q must place * at a path boundary", route)
			}
		}

		for i, p := range parts {
			parts[i] = regexp.QuoteMeta(p)
		}
		re, err := regexp.Compile(`\A` + strings.Join(parts, `.*`) + `\z`)
		if err != nil {
			return nil, fmt.Errorf("security.NewRouteMatcher: %w", err)
		}
		m.patterns = append(m.patterns, re)
	}

	return m, nil
}

func (m *RouteMatcher) Matches(requestPath string) bool {
	clean := path.Clean(requestPath)
	if _, ok := m.exact[clean]; ok {
		return true
	}
	for _, re := range m.patterns {
		if re.MatchString(clean) {
			return true
		}
	}

	return false
}

func (m *RouteMatcher) Allows(r *http.Request) bool {
	return m != nil && m.Matches(r.URL.Path)
}
