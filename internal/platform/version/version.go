package version

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var versionBytes string

var Version = strings.TrimSpace(versionBytes)
