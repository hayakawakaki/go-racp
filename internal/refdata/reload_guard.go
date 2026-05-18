package refdata

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

//nolint:govet // alignment varies per T
type ReloadGuard[T any] struct {
	snap       atomic.Pointer[T]
	reloadMu   sync.Mutex
	statusMu   sync.RWMutex
	lastReload time.Time
	lastError  string
}

func (g *ReloadGuard[T]) Load() *T {
	return g.snap.Load()
}

func (g *ReloadGuard[T]) Store(snap *T) {
	g.snap.Store(snap)
	g.recordSuccess()
}

func (g *ReloadGuard[T]) Reload(loader func() (*T, error)) error {
	if !g.reloadMu.TryLock() {
		return ErrReloadConflict
	}
	defer g.reloadMu.Unlock()

	if loader == nil {
		err := fmt.Errorf("refdata.ReloadGuard.Reload: %w", ErrParseFailed)
		g.recordFailure(err)
		return err
	}
	snap, err := loader()
	if err != nil {
		g.recordFailure(err)
		return fmt.Errorf("refdata.ReloadGuard.Reload: %w", err)
	}
	if snap == nil {
		err = fmt.Errorf("refdata.ReloadGuard.Reload: loader returned nil snapshot: %w", ErrParseFailed)
		g.recordFailure(err)
		return err
	}

	g.snap.Store(snap)
	g.recordSuccess()

	return nil
}

func (g *ReloadGuard[T]) LastReload() time.Time {
	g.statusMu.RLock()
	defer g.statusMu.RUnlock()

	return g.lastReload
}

func (g *ReloadGuard[T]) LastError() string {
	g.statusMu.RLock()
	defer g.statusMu.RUnlock()

	return g.lastError
}

func (g *ReloadGuard[T]) recordSuccess() {
	g.statusMu.Lock()
	defer g.statusMu.Unlock()
	g.lastReload = time.Now()
	g.lastError = ""
}

func (g *ReloadGuard[T]) recordFailure(err error) {
	g.statusMu.Lock()
	defer g.statusMu.Unlock()
	g.lastError = err.Error()
}
