// Package main registers all plugins with the plugin registry via blank imports.
// Each imported package's init function calls plugin.Register; server.Start
// then calls plugin.MountAll to wire every registered plugin into the HTTP mux.
package main

import (
	// Blank import triggers auth.init(), which registers the "auth" plugin.
	_ "github.com/hayakawakaki/go-racp/internal/auth"
)
