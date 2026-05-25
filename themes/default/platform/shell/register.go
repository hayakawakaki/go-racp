package shell

import "github.com/hayakawakaki/go-racp/internal/platform/httpx"

func init() {
	httpx.ActiveHeader = Header
}
