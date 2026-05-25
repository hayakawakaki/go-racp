package ui

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/a-h/templ"
)

//go:embed icons/*.svg
var iconFS embed.FS

type iconEntry struct {
	prefix        string
	suffix        string
	existingClass string
}

var (
	classAttrRe  = regexp.MustCompile(`\s+class\s*=\s*"([^"]*)"`)
	iconRegistry = loadIcons()
)

func loadIcons() map[string]iconEntry {
	entries, err := fs.ReadDir(iconFS, "icons")
	if err != nil {
		panic("ui: read icons fs: " + err.Error())
	}

	registry := make(map[string]iconEntry, len(entries))

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".svg") {
			continue
		}

		data, err := fs.ReadFile(iconFS, "icons/"+e.Name())
		if err != nil {
			panic("ui: read icon " + e.Name() + ": " + err.Error())
		}

		name := strings.TrimSuffix(e.Name(), ".svg")
		registry[name] = parseIconSVG(string(data))
	}

	return registry
}

func parseIconSVG(s string) iconEntry {
	start := strings.Index(s, "<svg")
	if start == -1 {
		return iconEntry{prefix: s}
	}

	rel := strings.Index(s[start:], ">")
	if rel == -1 {
		return iconEntry{prefix: s}
	}

	end := start + rel
	openTag := s[start : end+1]
	existingClass := ""

	if m := classAttrRe.FindStringSubmatch(openTag); m != nil {
		existingClass = m[1]
		openTag = classAttrRe.ReplaceAllString(openTag, "")
	}

	return iconEntry{
		prefix:        s[:start] + "<svg",
		suffix:        openTag[len("<svg"):] + s[end+1:],
		existingClass: existingClass,
	}
}

func Icon(name string, class ...string) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		entry, ok := iconRegistry[name]
		if !ok {
			return nil
		}

		extras := class
		if entry.existingClass != "" {
			extras = append([]string{entry.existingClass}, class...)
		}

		merged := MergeWithDefault("size-4", extras)
		if _, err := io.WriteString(w, entry.prefix+` class="`+templ.EscapeString(merged)+`"`+entry.suffix); err != nil {
			return fmt.Errorf("write icon: %w", err)
		}

		return nil
	})
}

func IconNames() []string {
	return slices.Sorted(maps.Keys(iconRegistry))
}
