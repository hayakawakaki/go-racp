package main

// blank import will run the init() and register it with the plugin registry.
// server.Start() later calls plugin.MountAll to wire them into the mux.

import (
	_ "github.com/hayakawakaki/go-racp/internal/features/account"
	_ "github.com/hayakawakaki/go-racp/internal/features/admin"
	_ "github.com/hayakawakaki/go-racp/internal/features/character"
	_ "github.com/hayakawakaki/go-racp/internal/features/guild"
	_ "github.com/hayakawakaki/go-racp/internal/features/item"
	_ "github.com/hayakawakaki/go-racp/internal/features/metric"
	_ "github.com/hayakawakaki/go-racp/internal/features/mob"
	_ "github.com/hayakawakaki/go-racp/internal/features/news"
	_ "github.com/hayakawakaki/go-racp/internal/features/stall"
	_ "github.com/hayakawakaki/go-racp/internal/features/tickets"
	_ "github.com/hayakawakaki/go-racp/internal/platform/theme"
	_ "github.com/hayakawakaki/go-racp/internal/platform/ui"
	_ "github.com/hayakawakaki/go-racp/themes/default/platform/shell"
)
