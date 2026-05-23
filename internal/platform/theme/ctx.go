package theme

import "net/http"

type Ctx struct {
	Req *http.Request
}

func BuildCtx(r *http.Request) Ctx {
	return Ctx{Req: r}
}
