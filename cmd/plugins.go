package main

// blank import will run the init() and register it with the plugin registry.
// server.Start() later calls plugin.MountAll to wire them into the mux.
import (
	_ "github.com/hayakawakaki/go-racp/internal/auth"
)
