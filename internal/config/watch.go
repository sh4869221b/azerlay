package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/fswatcher/fswatcher"
)

// Start registers the parent directory before loading the initial configuration.
// The returned manager owns the watch until ctx is cancelled.
func Start(ctx context.Context, explicitPath string) (*Manager, error) {
	path, err := ResolvePath(explicitPath)
	if err != nil {
		return nil, err
	}
	w, target, err := openConfigWatch(path)
	if err != nil {
		return nil, err
	}
	m, ready := attachConfigWatch(ctx, w, configWatch{target: target, events: w.Events, errors: w.Errors}, func(context.Context) (Config, []Warning, error) { return loadFile(target) })
	if err := <-ready; err != nil {
		<-m.Done()
		return nil, err
	}
	return m, nil
}

func openConfigWatch(path string) (*fswatcher.Watcher, string, error) {
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return nil, "", watchError(err)
	}
	w, err := fswatcher.NewWatcher()
	if err != nil {
		return nil, "", watchError(err)
	}
	if err := w.Add(parent, fswatcher.Create|fswatcher.Write|fswatcher.Remove|fswatcher.Rename); err != nil {
		return nil, "", watchError(errors.Join(err, w.Close()))
	}
	return w, filepath.Join(parent, filepath.Base(path)), nil
}

func watchError(err error) *Error {
	code := ERR_CONFIG_INVALID
	if errors.Is(err, os.ErrNotExist) {
		code = ERR_CONFIG_NOT_FOUND
	}
	return &Error{Code: code, Stage: "watch", Reason: "configuration watch failed", Cause: err}
}

type configWatch struct {
	target string
	events <-chan fswatcher.Event
	errors <-chan error
}

func attachConfigWatch(ctx context.Context, w *fswatcher.Watcher, watch configWatch, load configLoader) (*Manager, <-chan error) {
	ctx, cancel := context.WithCancel(ctx)
	events := make(chan managerEvent)
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		defer w.Close()
		watch.run(ctx, events)
	}()
	m := &Manager{ctx: ctx, events: events, changes: make(chan struct{}, 1), done: make(chan struct{})}
	ready := make(chan error, 1)
	go func() {
		m.run(ctx, load, events, ready)
		cancel()
		<-watchDone
		close(m.done)
	}()
	return m, ready
}

func (w configWatch) run(ctx context.Context, out chan<- managerEvent) {
	timer := time.NewTimer(200 * time.Millisecond)
	timer.Stop()
	defer timer.Stop()
	var quiet <-chan time.Time
	send := func(event managerEvent) bool {
		select {
		case out <- event:
			return true
		case <-ctx.Done():
			return false
		}
	}
	failure := managerEvent{kind: reportWatchFailure, failure: Diagnostic{Code: ERR_CONFIG_INVALID, Stage: "watch", Reason: "configuration watch failed"}}
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.events:
			if !ok {
				w.events = nil
				if !send(failure) {
					return
				}
				continue
			}
			request, relevant := w.classify(event)
			if !relevant {
				continue
			}
			if !send(request) {
				return
			}
			if request.kind == observeReload {
				timer.Reset(200 * time.Millisecond)
				quiet = timer.C
			}
		case _, ok := <-w.errors:
			if !ok {
				w.errors = nil
			}
			if !send(failure) {
				return
			}
		case <-quiet:
			quiet = nil
			if !send(managerEvent{kind: releaseReload}) {
				return
			}
		}
	}
}

func (w configWatch) classify(event fswatcher.Event) (managerEvent, bool) {
	if event.Name == filepath.Dir(w.target) && event.Op&(fswatcher.Remove|fswatcher.Rename) != 0 {
		return managerEvent{kind: reportWatchFailure, failure: Diagnostic{Code: ERR_CONFIG_INVALID, Stage: "watch", Reason: "configuration watch parent moved or removed"}}, true
	}
	if event.Name == w.target && event.Op&(fswatcher.Create|fswatcher.Write|fswatcher.Remove|fswatcher.Rename) != 0 {
		return managerEvent{kind: observeReload}, true
	}
	return managerEvent{}, false
}
