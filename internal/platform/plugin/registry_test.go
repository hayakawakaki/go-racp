package plugin

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
)

func resetRegistry(t *testing.T) {
	t.Helper()
	prevRegistry := registry
	prevMounted := mounted
	registry = nil
	mounted = false
	t.Cleanup(func() {
		registry = prevRegistry
		mounted = prevMounted
	})
}

func testInfra() *infra.Infra {
	return &infra.Infra{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func noopMount(_ *routes.Registry, _ *http.ServeMux, _ *infra.Infra) {}

func noopMiddleware(_ *infra.Infra, h http.Handler) http.Handler { return h }

func expectPanic(t *testing.T, substring string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic containing %q, got none", substring)
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value type = %T, want string: %v", r, r)
		}
		if !strings.Contains(msg, substring) {
			t.Errorf("panic = %q, want substring %q", msg, substring)
		}
	}()
	fn()
}

func TestRegister_PanicsWithoutName(t *testing.T) {
	resetRegistry(t)
	expectPanic(t, "Name required", func() {
		Register(Plugin{Mount: noopMount})
	})
}

func TestRegister_PanicsWithoutMountOrMiddleware(t *testing.T) {
	resetRegistry(t)
	expectPanic(t, "Mount or Middleware required", func() {
		Register(Plugin{Name: "empty"})
	})
}

func TestRegister_AcceptsMountOnly(t *testing.T) {
	resetRegistry(t)
	Register(Plugin{Name: "mount-only", Mount: noopMount})
	if len(registry) != 1 {
		t.Errorf("registry len = %d, want 1", len(registry))
	}
}

func TestRegister_AcceptsMiddlewareOnly(t *testing.T) {
	resetRegistry(t)
	Register(Plugin{Name: "middleware-only", Middleware: noopMiddleware})
	if len(registry) != 1 {
		t.Errorf("registry len = %d, want 1", len(registry))
	}
}

func TestRegister_AcceptsBoth(t *testing.T) {
	resetRegistry(t)
	Register(Plugin{Name: "both", Mount: noopMount, Middleware: noopMiddleware})
	if len(registry) != 1 {
		t.Errorf("registry len = %d, want 1", len(registry))
	}
}

func TestRegister_PanicsOnDuplicateName(t *testing.T) {
	resetRegistry(t)
	Register(Plugin{Name: "dup", Mount: noopMount})
	expectPanic(t, "duplicate plugin name: dup", func() {
		Register(Plugin{Name: "dup", Mount: noopMount})
	})
}

func TestRegister_PanicsAfterMountAll(t *testing.T) {
	resetRegistry(t)
	Register(Plugin{Name: "first", Mount: noopMount})
	MountAll(&routes.Registry{}, http.NewServeMux(), testInfra())
	expectPanic(t, "Register called after MountAll: late", func() {
		Register(Plugin{Name: "late", Mount: noopMount})
	})
}

func TestMountAll_InvokesMountInRegistrationOrder(t *testing.T) {
	resetRegistry(t)
	var mountOrder []string
	Register(Plugin{Name: "alpha", Mount: func(_ *routes.Registry, _ *http.ServeMux, _ *infra.Infra) {
		mountOrder = append(mountOrder, "alpha")
	}})
	Register(Plugin{Name: "beta", Mount: func(_ *routes.Registry, _ *http.ServeMux, _ *infra.Infra) {
		mountOrder = append(mountOrder, "beta")
	}})
	Register(Plugin{Name: "gamma", Mount: func(_ *routes.Registry, _ *http.ServeMux, _ *infra.Infra) {
		mountOrder = append(mountOrder, "gamma")
	}})

	MountAll(&routes.Registry{}, http.NewServeMux(), testInfra())

	want := []string{"alpha", "beta", "gamma"}
	if len(mountOrder) != len(want) {
		t.Fatalf("mountOrder = %v, want %v", mountOrder, want)
	}
	for index, name := range want {
		if mountOrder[index] != name {
			t.Errorf("mountOrder[%d] = %q, want %q", index, mountOrder[index], name)
		}
	}
}

func TestMountAll_SkipsPluginsWithoutMount(t *testing.T) {
	resetRegistry(t)
	mounted := false
	Register(Plugin{Name: "middleware-only", Middleware: noopMiddleware})
	Register(Plugin{Name: "has-mount", Mount: func(_ *routes.Registry, _ *http.ServeMux, _ *infra.Infra) {
		mounted = true
	}})

	MountAll(&routes.Registry{}, http.NewServeMux(), testInfra())

	if !mounted {
		t.Errorf("Mount-bearing plugin was not invoked")
	}
}

func TestMountAll_PanicsOnSecondCall(t *testing.T) {
	resetRegistry(t)
	Register(Plugin{Name: "x", Mount: noopMount})
	MountAll(&routes.Registry{}, http.NewServeMux(), testInfra())
	expectPanic(t, "MountAll called more than once", func() {
		MountAll(&routes.Registry{}, http.NewServeMux(), testInfra())
	})
}

func TestMiddlewares_ReturnsOnlyMiddlewareBearingPlugins(t *testing.T) {
	resetRegistry(t)
	Register(Plugin{Name: "mount-only", Mount: noopMount})
	Register(Plugin{Name: "middleware-only", Middleware: noopMiddleware})
	Register(Plugin{Name: "both", Mount: noopMount, Middleware: noopMiddleware})

	got := Middlewares()
	if len(got) != 2 {
		t.Fatalf("Middlewares len = %d, want 2", len(got))
	}
	names := []string{got[0].Name, got[1].Name}
	if names[0] != "middleware-only" || names[1] != "both" {
		t.Errorf("Middlewares names = %v, want [middleware-only both]", names)
	}
}

func TestMiddlewares_EmptyWhenNonePresent(t *testing.T) {
	resetRegistry(t)
	Register(Plugin{Name: "a", Mount: noopMount})
	Register(Plugin{Name: "b", Mount: noopMount})

	got := Middlewares()
	if len(got) != 0 {
		t.Errorf("Middlewares len = %d, want 0", len(got))
	}
}

func TestMiddlewares_PreservesRegistrationOrder(t *testing.T) {
	resetRegistry(t)
	Register(Plugin{Name: "first", Middleware: noopMiddleware})
	Register(Plugin{Name: "between", Mount: noopMount})
	Register(Plugin{Name: "second", Middleware: noopMiddleware})

	got := Middlewares()
	if len(got) != 2 {
		t.Fatalf("Middlewares len = %d, want 2", len(got))
	}
	if got[0].Name != "first" || got[1].Name != "second" {
		t.Errorf("order = [%s %s], want [first second]", got[0].Name, got[1].Name)
	}
}
