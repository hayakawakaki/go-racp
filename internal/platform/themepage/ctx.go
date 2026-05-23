package themepage

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

type Ctx struct {
	Req    *http.Request
	Layout httpx.Layout
}

func BuildCtx(r *http.Request, layout httpx.Layout) Ctx {
	return Ctx{Req: r, Layout: layout}
}
