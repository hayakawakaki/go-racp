package ui

import (
	"maps"

	twmerge "github.com/Oudwins/tailwind-merge-go"
	"github.com/a-h/templ"
)

func Merge(classes ...string) string {
	return twmerge.Merge(classes...)
}

func MergeAttr(attributes ...templ.Attributes) templ.Attributes {
	merged := templ.Attributes{}

	for _, attr := range attributes {
		maps.Copy(merged, attr)
	}

	return merged
}
