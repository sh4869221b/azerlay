package profilesource

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/fswatcher/fswatcher"
)

func nextLocalChange(t *testing.T, changes <-chan SourceChange) SourceChange {
	t.Helper()
	select {
	case change, ok := <-changes:
		if !ok {
			t.Fatal("watch closed without change")
		}
		return change
	case <-time.After(5 * time.Second):
		t.Fatal("watch change timed out")
	}
	return SourceChange{}
}
func stopLocalWatch(t *testing.T, cancel context.CancelFunc, changes <-chan SourceChange) {
	t.Helper()
	cancel()
	select {
	case _, ok := <-changes:
		if ok {
			t.Fatal("cancel published change")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watch shutdown timed out")
	}
}
func TestLocalWatchRealWriteRenameRepair(t *testing.T) {
	t.Parallel()
	s, ref, path := localFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	changes, err := s.Watch(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	defer stopLocalWatch(t, cancel, changes)
	assertName := func(want string) {
		t.Helper()
		got := nextLocalChange(t, changes)
		if got.Ref != ref || got.Failure != nil || got.Candidate == nil || got.Candidate.Bundle().Profiles[0].Name == nil || *got.Candidate.Bundle().Profiles[0].Name != want {
			t.Fatalf("candidate: %+v", got)
		}
	}
	assertName("Synthetic")
	replacement := func(name string) []byte { return []byte(strings.Replace(savedLocalJSON, "Synthetic", name, 1)) }
	if err := os.WriteFile(path, replacement("Written"), 0600); err != nil {
		t.Fatal(err)
	}
	assertName("Written")
	sibling := filepath.Join(filepath.Dir(path), "writer.tmp")
	if err := os.WriteFile(sibling, replacement("Renamed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(sibling, path); err != nil {
		t.Fatal(err)
	}
	assertName("Renamed")
	if err := os.WriteFile(path, []byte(`{"id":`), 0600); err != nil {
		t.Fatal(err)
	}
	got := nextLocalChange(t, changes)
	if got.Candidate != nil || got.Failure == nil || got.Failure.Code != ERR_PROFILE_LOCAL_UNSUPPORTED || got.Failure.Cause != nil {
		t.Fatalf("partial: %+v", got)
	}
	if err := os.WriteFile(path, replacement("Repaired"), 0600); err != nil {
		t.Fatal(err)
	}
	assertName("Repaired")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	got = nextLocalChange(t, changes)
	if got.Candidate != nil || got.Failure == nil || got.Failure.Code != ERR_PROFILE_LOCAL_READ {
		t.Fatalf("removed: %+v", got)
	}
	if err := os.WriteFile(path, replacement("Recreated"), 0600); err != nil {
		t.Fatal(err)
	}
	assertName("Recreated")
	if err := os.WriteFile(sibling, replacement("Ignored"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-changes:
		t.Fatalf("unrelated publication: %+v", got)
	case <-time.After(500 * time.Millisecond):
	}
}
func TestLocalWatchParentLossAndSetup(t *testing.T) {
	t.Parallel()
	s, ref, path := localFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	changes, err := s.Watch(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	nextLocalChange(t, changes)
	moved := filepath.Join(s.root, "moved")
	if err := os.Rename(filepath.Dir(path), moved); err != nil {
		t.Fatal(err)
	}
	got := nextLocalChange(t, changes)
	if got.Candidate != nil || got.Failure == nil || got.Failure.Code != ERR_PROFILE_LOCAL_WATCH {
		t.Fatalf("parent loss: %+v", got)
	}
	if _, ok := <-changes; ok {
		t.Fatal("failed watch did not close")
	}
	if got, err := s.Watch(ctx, ref); got != nil || err == nil || err.Error() != ERR_PROFILE_LOCAL_WATCH {
		t.Fatalf("missing parent: %v %v", got, err)
	}
	if got, err := s.Watch(ctx, SourceRef{Hash: "wrong"}); got != nil || err == nil || err.Error() != ERR_PROFILE_LOCAL_WATCH {
		t.Fatalf("wrong ref: %v %v", got, err)
	}
}

func TestLocalWatchQuietRetryBound(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		events := make(chan fswatcher.Event)
		target := "/source/profile_a.json"
		calls, closed := 0, false
		var starts []time.Time
		changes := runLocalWatch(ctx, SourceRef{}, target, events, nil, func() error { closed = true; return nil }, func() bool { return true }, func(context.Context, SourceRef) (LocalCandidate, error) {
			calls++
			starts = append(starts, time.Now())
			return LocalCandidate{}, &Error{Code: ERR_PROFILE_LOCAL_READ, Cause: errors.New("private")}
		})
		got := nextLocalChange(t, changes)
		if calls != 3 || got.Failure == nil || got.Failure.Code != ERR_PROFILE_LOCAL_READ || got.Failure.Cause != nil || starts[1].Sub(starts[0]) != 200*time.Millisecond || starts[2].Sub(starts[1]) != 200*time.Millisecond {
			t.Fatalf("bounded retries: calls=%d starts=%v change=%+v", calls, starts, got)
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if calls != 3 {
			t.Fatal("retry continued after exhaustion")
		}
		events <- fswatcher.Event{Name: "/source/unrelated", Op: fswatcher.Write}
		events <- fswatcher.Event{Name: target, Op: fswatcher.Chmod}
		synctest.Wait()
		if calls != 3 {
			t.Fatal("unrelated event loaded")
		}
		begin := time.Now()
		events <- fswatcher.Event{Name: target, Op: fswatcher.Write}
		time.Sleep(50 * time.Millisecond)
		events <- fswatcher.Event{Name: target, Op: fswatcher.Write}
		nextLocalChange(t, changes)
		if calls != 6 || starts[3].Sub(begin) != 250*time.Millisecond {
			t.Fatalf("quiet/retry: calls=%d starts=%v", calls, starts)
		}
		stopLocalWatch(t, cancel, changes)
		if !closed {
			t.Fatal("watch not joined")
		}
	})
}
func TestLocalWatchStaleResults(t *testing.T) {
	t.Parallel()
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[failure], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				events := make(chan fswatcher.Event)
				release := make(chan struct{})
				calls := 0
				changes := runLocalWatch(ctx, SourceRef{}, "/source/profile_a.json", events, nil, func() error { return nil }, func() bool { return true }, func(ctx context.Context, _ SourceRef) (LocalCandidate, error) {
					calls++
					if calls == 1 {
						<-release
						if failure {
							return LocalCandidate{}, &Error{Code: ERR_PROFILE_LOCAL_READ}
						}
						return LocalCandidate{original: []byte("stale")}, nil
					}
					return LocalCandidate{original: []byte("latest")}, nil
				})
				synctest.Wait()
				events <- fswatcher.Event{Name: "/source/profile_a.json", Op: fswatcher.Write}
				time.Sleep(200 * time.Millisecond)
				if calls != 1 {
					t.Fatal("overlapping loads")
				}
				close(release)
				got := nextLocalChange(t, changes)
				if calls != 2 || got.Failure != nil || got.Candidate == nil || string(got.Candidate.Original()) != "latest" {
					t.Fatalf("stale publication: %+v calls=%d", got, calls)
				}
				stopLocalWatch(t, cancel, changes)
			})
		})
	}
}
func TestLocalWatchFailureAndCancelJoin(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"error", "events_closed", "errors_closed", "parent", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				events := make(chan fswatcher.Event)
				failures := make(chan error)
				closed, joined, alive := false, false, true
				changes := runLocalWatch(ctx, SourceRef{}, "/source/profile_a.json", events, failures, func() error { closed = true; return nil }, func() bool { return alive }, func(ctx context.Context, _ SourceRef) (LocalCandidate, error) {
					<-ctx.Done()
					joined = true
					return LocalCandidate{}, ctx.Err()
				})
				synctest.Wait()
				switch mode {
				case "error":
					failures <- errors.New("private watch cause")
				case "events_closed":
					close(events)
				case "errors_closed":
					close(failures)
				case "parent":
					alive = false
					events <- fswatcher.Event{Name: "/source", Op: fswatcher.Rename}
				case "cancel":
					cancel()
				}
				if mode != "cancel" {
					got := nextLocalChange(t, changes)
					if got.Candidate != nil || got.Failure == nil || got.Failure.Code != ERR_PROFILE_LOCAL_WATCH || got.Failure.Cause != nil {
						t.Fatalf("watch failure: %+v", got)
					}
				}
				if _, ok := <-changes; ok {
					t.Fatal("channel did not close")
				}
				if !closed || !joined {
					t.Fatalf("shutdown not joined: close=%v load=%v", closed, joined)
				}
			})
		})
	}
}

func TestLocalWatchRegisteredBeforeInitialRead(t *testing.T) {
	t.Parallel()
	s, ref, path := localFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	first := true
	s.hook = func(phase string) {
		if phase == "between" && first {
			first = false
			close(entered)
			<-release
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	changes, err := s.Watch(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		cancel()
		close(release)
		t.Fatal("initial read did not start")
	}
	// The parent must already be registered while the initial stable pair is
	// paused; this edit must invalidate that request and produce the latest pair.
	if err := os.WriteFile(path, []byte(strings.Replace(savedLocalJSON, "Synthetic", "Startup latest", 1)), 0600); err != nil {
		cancel()
		close(release)
		t.Fatal(err)
	}
	close(release)
	defer stopLocalWatch(t, cancel, changes)
	got := nextLocalChange(t, changes)
	if got.Failure != nil || got.Candidate == nil || got.Candidate.Bundle().Profiles[0].Name == nil || *got.Candidate.Bundle().Profiles[0].Name != "Startup latest" {
		t.Fatalf("startup edit: %+v", got)
	}
}
