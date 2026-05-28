package transport

import (
	"net/url"

	guildstate "github.com/hayakawakaki/go-racp/internal/features/guild/transport/state"
)

func pageHrefPattern(state guildstate.ListState) string {
	values := url.Values{}
	values.Set("page", "__PAGE__")
	if state.Query != "" {
		values.Set("q", state.Query)
	}
	base := state.BaseURL
	if base == "" {
		base = "/admin/guilds"
	}

	return base + "?" + values.Encode()
}
