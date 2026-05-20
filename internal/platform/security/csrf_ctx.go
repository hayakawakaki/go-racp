package security

import "context"

type csrfCtxKey int

const csrfTokenKey csrfCtxKey = 0

func TokenFromContext(ctx context.Context) string {
	v, _ := ctx.Value(csrfTokenKey).(string)
	return v
}

func contextWithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfTokenKey, token)
}

func HxHeadersValue(ctx context.Context) string {
	return `{"X-CSRF-Token":"` + TokenFromContext(ctx) + `"}`
}
