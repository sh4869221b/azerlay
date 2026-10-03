package profilesource

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/fswatcher/fswatcher"
)

// Watch registers the selected parent before the initial load. Closing changes
// guarantees that the watcher and active load have stopped. Cancel and drain
// changes to join shutdown; cancellation itself never reports a failure.
func (s *AzeronLocalSource) Watch(ctx context.Context, ref SourceRef) (<-chan SourceChange, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !validLocalRef(ref) || ref.Local.Root != s.root {
		return nil, &Error{Code: ERR_PROFILE_LOCAL_WATCH}
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return nil, &Error{Code: ERR_PROFILE_LOCAL_WATCH}
	}
	defer root.Close()
	relative := filepath.Dir(localRelativePath(ref.Local))
	info, err := root.Stat(relative)
	if err != nil || !info.IsDir() {
		return nil, &Error{Code: ERR_PROFILE_LOCAL_WATCH}
	}
	parent, err := filepath.EvalSymlinks(filepath.Join(s.root, relative))
	if err != nil {
		return nil, &Error{Code: ERR_PROFILE_LOCAL_WATCH}
	}
	w, err := fswatcher.NewWatcher()
	if err != nil {
		return nil, &Error{Code: ERR_PROFILE_LOCAL_WATCH}
	}
	if err := w.Add(parent, fswatcher.Create|fswatcher.Write|fswatcher.Remove|fswatcher.Rename); err != nil {
		w.Close()
		return nil, &Error{Code: ERR_PROFILE_LOCAL_WATCH}
	}
	alive := func() bool {
		root, err := os.OpenRoot(s.root)
		if err != nil {
			return false
		}
		defer root.Close()
		current, err := root.Stat(relative)
		return err == nil && os.SameFile(info, current)
	}
	return runLocalWatch(ctx, ref, filepath.Join(parent, ref.Local.File), w.Events, w.Errors, w.Close, alive, s.LoadCandidate), nil
}

type localLoadResult struct {
	generation uint64
	candidate  LocalCandidate
	err        error
}

// One coordinator owns quieting, retry scheduling and publication. A relevant
// event invalidates the active result immediately, before the quiet period ends.
func runLocalWatch(ctx context.Context, ref SourceRef, target string, events <-chan fswatcher.Event, failures <-chan error, closeWatch func() error, alive func() bool, load func(context.Context, SourceRef) (LocalCandidate, error)) <-chan SourceChange {
	changes := make(chan SourceChange)
	go func() {
		defer close(changes)
		cancel := func() {}
		results := make(chan localLoadResult, 1)
		active := false
		defer func() {
			cancel()
			closeWatch()
			if active {
				<-results
			}
		}()
		timer := time.NewTimer(localReadInterval)
		timer.Stop()
		defer timer.Stop()
		var wake <-chan time.Time
		generation := uint64(1)
		attempts := 0
		pending := true
		ready := true
		var publication *SourceChange
		watchFailed := false
		observe := func(event fswatcher.Event) {
			if !alive() || event.Name == filepath.Dir(target) && event.Op&(fswatcher.Remove|fswatcher.Rename) != 0 {
				watchFailed = true
				return
			}
			if event.Name == target && event.Op&(fswatcher.Create|fswatcher.Write|fswatcher.Remove|fswatcher.Rename) != 0 {
				generation++
				publication = nil
				cancel()
				attempts = 0
				pending, ready = true, false
				timer.Reset(localReadInterval)
				wake = timer.C
			}
		}
		for {
			if ctx.Err() != nil {
				return
			}
			if watchFailed {
				cancel()
				closeWatch()
				if active {
					<-results
					active = false
				}
				select {
				case changes <- SourceChange{Ref: ref, Failure: &Error{Code: ERR_PROFILE_LOCAL_WATCH}}:
				case <-ctx.Done():
				}
				return
			}
			if pending && ready && !active {
				pending, ready, active = false, false, true
				attempts++
				loadCtx, stop := context.WithCancel(ctx)
				cancel = stop
				request := generation
				go func(ctx context.Context) {
					defer stop()
					candidate, err := load(ctx, ref)
					results <- localLoadResult{request, candidate, err}
				}(loadCtx)
			}
			var output chan SourceChange
			var value SourceChange
			if publication != nil {
				output, value = changes, *publication
			}
			select {
			case <-ctx.Done():
				return
			case event, ok := <-events:
				if !ok {
					watchFailed = true
				} else {
					observe(event)
				}
			case <-failures:
				watchFailed = true
			case <-wake:
				wake = nil
				ready = true
			case result := <-results:
				active = false
				cancel()
				// Consume queued notifications before considering an older result.
			drain:
				for {
					select {
					case event, ok := <-events:
						if !ok {
							watchFailed = true
							break drain
						}
						observe(event)
					case <-failures:
						watchFailed = true
						break drain
					default:
						break drain
					}
				}
				if watchFailed || result.generation != generation {
					continue
				}
				if !alive() {
					watchFailed = true
					continue
				}
				if result.err == nil {
					publication = &SourceChange{Ref: ref, Candidate: &result.candidate}
				} else if attempts < 3 {
					pending = true
					timer.Reset(localReadInterval)
					wake = timer.C
				} else {
					failure, ok := result.err.(*Error)
					if !ok {
						failure = &Error{Code: ERR_PROFILE_LOCAL_READ}
					}
					// Expose only the safe code, never an operating-system cause.
					publication = &SourceChange{Ref: ref, Failure: &Error{Code: failure.Code}}
				}
			case output <- value:
				publication = nil
			}
		}
	}()
	return changes
}
