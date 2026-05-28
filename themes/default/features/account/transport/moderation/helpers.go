package moderation

import (
	"net/url"

	accountmoderationstate "github.com/hayakawakaki/go-racp/internal/features/account/transport/moderation/state"
)

func pageHrefPattern(state accountmoderationstate.ListState) string {
	values := url.Values{}
	values.Set("page", "__PAGE__")
	if state.Query != "" {
		values.Set("q", state.Query)
	}
	base := state.BaseURL
	if base == "" {
		base = "/admin/users"
	}

	return base + "?" + values.Encode()
}
