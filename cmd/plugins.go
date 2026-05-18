package main

// blank import will run the init() and register it with the plugin registry.
// server.Start() later calls plugin.MountAll to wire them into the mux.
import (
	_ "github.com/hayakawakaki/go-racp/internal/account"
	_ "github.com/hayakawakaki/go-racp/internal/admin"
	_ "github.com/hayakawakaki/go-racp/internal/character"
	_ "github.com/hayakawakaki/go-racp/internal/home"
	_ "github.com/hayakawakaki/go-racp/internal/item"
	_ "github.com/hayakawakaki/go-racp/internal/mob"
	_ "github.com/hayakawakaki/go-racp/internal/news"
	_ "github.com/hayakawakaki/go-racp/internal/tickets"
)
