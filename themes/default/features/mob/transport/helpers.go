package transport

import (
	"net/url"

	mobstate "github.com/hayakawakaki/go-racp/internal/features/mob/transport/state"
)

func pageHrefPattern(state mobstate.ListState) string {
	values := url.Values{}
	values.Set("page", "__PAGE__")
	if state.Query != "" {
		values.Set("q", state.Query)
	}
	base := state.BaseURL
	if base == "" {
		base = "/mobs"
	}

	return base + "?" + values.Encode()
}
