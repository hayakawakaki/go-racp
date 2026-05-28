package ui

import (
	"maps"
	"sync"

	twmerge "github.com/Oudwins/tailwind-merge-go"
	"github.com/a-h/templ"
)

var mergeMu sync.Mutex

func Merge(classes ...string) string {
	mergeMu.Lock()
	defer mergeMu.Unlock()

	return twmerge.Merge(classes...)
}

func MergeWithDefault(defaults string, extras []string) string {
	return Merge(append([]string{defaults}, extras...)...)
}

func MergeAttr(attributes ...templ.Attributes) templ.Attributes {
	merged := templ.Attributes{}

	for _, attr := range attributes {
		maps.Copy(merged, attr)
	}

	return merged
}
