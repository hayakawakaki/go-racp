package transport

import (
	"fmt"
	"net/url"

	mobstate "github.com/hayakawakaki/go-racp/internal/features/mob/transport/state"
)

func pageURL(state mobstate.ListState, page int) string {
	values := url.Values{}
	values.Set("page", fmt.Sprintf("%d", page))
	if state.Query != "" {
		values.Set("q", state.Query)
	}
	base := state.BaseURL
	if base == "" {
		base = "/mobs"
	}

	return base + "?" + values.Encode()
}
