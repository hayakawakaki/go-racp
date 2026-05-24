package ui

import (
	"maps"

	twmerge "github.com/Oudwins/tailwind-merge-go"
	"github.com/a-h/templ"
)

func Merge(classes ...string) string {
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
