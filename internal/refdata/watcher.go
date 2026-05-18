package refdata

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type ReloadFunc func(context.Context) error

type Watcher struct {
	fs        *fsnotify.Watcher
	logger    *slog.Logger
	closeErr  error
	reload    ReloadFunc
	targets   map[string]struct{}
	closeOnce sync.Once
	debounce  time.Duration
}

const defaultWatcherDebounce = 300 * time.Millisecond

func StartWatcher(ctx context.Context, paths []string, reload ReloadFunc, logger *slog.Logger) (*Watcher, error) {
	if len(paths) == 0 || reload == nil {
		return nil, nil
	}
	if logger == nil {
		logger = slog.Default()
	}

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("refdata.StartWatcher: %w", err)
	}

	targets := make(map[string]struct{}, len(paths))
	dirs := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		abs, absErr := filepath.Abs(path)
		if absErr != nil {
			continue
		}
		targets[abs] = struct{}{}
		dirs[filepath.Dir(abs)] = struct{}{}
	}

	for dir := range dirs {
		if err := fsWatcher.Add(dir); err != nil {
			logger.Warn("refdata: watcher add failed", "dir", dir, "err", err)
		}
	}

	watcher := &Watcher{
		fs:       fsWatcher,
		logger:   logger,
		reload:   reload,
		targets:  targets,
		debounce: defaultWatcherDebounce,
	}
	go watcher.run(ctx)

	return watcher, nil
}

func (w *Watcher) Close() error {
	if w == nil || w.fs == nil {
		return nil
	}
	w.closeOnce.Do(func() {
		if err := w.fs.Close(); err != nil {
			w.closeErr = fmt.Errorf("refdata.Watcher.Close: %w", err)
		}
	})

	return w.closeErr
}

func (w *Watcher) run(ctx context.Context) {
	defer func() { _ = w.Close() }()

	var timer *time.Timer
	fire := func() {
		if err := w.reload(ctx); err != nil {
			w.logger.Warn("refdata: watcher reload failed", "err", err)

			return
		}
		w.logger.Info("refdata: snapshot reloaded via watcher")
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.fs.Events:
			if !ok {
				return
			}
			if !w.matches(event) {
				continue
			}
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(w.debounce, fire)
		case err, ok := <-w.fs.Errors:
			if !ok {
				return
			}
			w.logger.Warn("refdata: watcher error", "err", err)
		}
	}
}

func (w *Watcher) matches(event fsnotify.Event) bool {
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
		return false
	}
	abs, err := filepath.Abs(event.Name)
	if err != nil {
		return false
	}
	_, hit := w.targets[abs]

	return hit
}
