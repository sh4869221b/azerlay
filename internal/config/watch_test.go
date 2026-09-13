package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/fswatcher/fswatcher"
)

func writeWatchConfig(t *testing.T, path, name string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(fmt.Sprintf("schema_version = 1\n[profile]\nselected_id = %q\nwatch = false\n", name)), 0600); err != nil {
		t.Fatal(err)
	}
}

func watchFixture(t *testing.T) (string, *Manager, context.Context, context.CancelFunc) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	writeWatchConfig(t, path, "A")
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	m, err := Start(ctx, path)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); waitWatchDone(t, m.Done()) })
	return path, m, ctx, cancel
}

func waitWatchDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("watch did not stop")
	}
}

func awaitWatchName(t *testing.T, ctx context.Context, m *Manager, name string) Snapshot {
	t.Helper()
	return awaitSnapshot(t, ctx, m, func(s Snapshot) bool {
		return s.Config.Profile.SelectedID == name && s.Status.ConfigFailure == (Diagnostic{})
	})
}

func TestConfigWatchWrite(t *testing.T) {
	t.Parallel()
	path, m, ctx, _ := watchFixture(t)
	started := time.Now()
	writeWatchConfig(t, path, "B")
	awaitWatchName(t, ctx, m, "B")
	elapsed := time.Since(started)
	if elapsed < 200*time.Millisecond {
		t.Fatalf("edit reloaded before quiet period: %v", elapsed)
	}
	t.Logf("write recovery: %v", elapsed)
}

func TestConfigWatchRename(t *testing.T) {
	t.Parallel()
	path, m, ctx, _ := watchFixture(t)
	sibling := filepath.Join(filepath.Dir(path), "replacement.toml")
	writeWatchConfig(t, sibling, "replacement")
	started := time.Now()
	if err := os.Rename(sibling, path); err != nil {
		t.Fatal(err)
	}
	awaitWatchName(t, ctx, m, "replacement")
	t.Logf("atomic rename recovery: %v", time.Since(started))
}

func TestConfigWatchRecovery(t *testing.T) {
	t.Parallel()
	path, m, ctx, _ := watchFixture(t)
	if err := os.WriteFile(path, []byte("schema_version = ["), 0600); err != nil {
		t.Fatal(err)
	}
	s := awaitSnapshot(t, ctx, m, func(s Snapshot) bool { return s.Status.ConfigFailure.Code == ERR_CONFIG_INVALID })
	if s.Config.Profile.SelectedID != "A" || s.Status.ConfigGeneration != 1 {
		t.Fatal("invalid edit replaced good config")
	}
	started := time.Now()
	writeWatchConfig(t, path, "repaired")
	awaitWatchName(t, ctx, m, "repaired")
	t.Logf("invalid-to-valid recovery: %v", time.Since(started))
}

func TestConfigWatchRecreate(t *testing.T) {
	t.Parallel()
	path, m, ctx, _ := watchFixture(t)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	s := awaitSnapshot(t, ctx, m, func(s Snapshot) bool { return s.Status.ConfigFailure.Code == ERR_CONFIG_NOT_FOUND })
	if s.Config.Profile.SelectedID != "A" {
		t.Fatal("removal replaced good config")
	}
	writeWatchConfig(t, path, "recreated")
	awaitWatchName(t, ctx, m, "recreated")
}

func TestConfigWatchStartupChange(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.toml")
	writeWatchConfig(t, path, "obsolete")
	w, target, err := openConfigWatch(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	loaded, release := make(chan struct{}), make(chan struct{})
	first := true
	m, ready := attachConfigWatch(ctx, w, configWatch{target: target, events: w.Events, errors: w.Errors}, func(ctx context.Context) (Config, []Warning, error) {
		c, warnings, err := loadFile(path)
		if first {
			first = false
			close(loaded)
			select {
			case <-release:
			case <-ctx.Done():
				return Config{}, nil, ctx.Err()
			}
		}
		return c, warnings, err
	})
	t.Cleanup(func() { cancel(); waitWatchDone(t, m.Done()) })
	select {
	case <-loaded:
	case <-ctx.Done():
		t.Fatal("initial load not started")
	}
	writeWatchConfig(t, path, "latest")
	awaitSnapshot(t, ctx, m, func(s Snapshot) bool { return s.Status.RequestGeneration > 1 })
	close(release)
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("startup timed out")
	}
	if s := m.Snapshot(); s.Config.Profile.SelectedID != "latest" || s.Status.ConfigGeneration <= 1 {
		t.Fatalf("stale startup published: %+v", s)
	}
}

func TestConfigWatchMissing(t *testing.T) {
	countWatches := func() int {
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, entry := range entries {
			name, _ := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
			if name == "anon_inode:inotify" {
				count++
			}
		}
		return count
	}
	before := countWatches()
	defer func() {
		if after := countWatches(); after != before {
			t.Fatalf("Start leaked inotify descriptors: before=%d after=%d", before, after)
		}
	}()
	for _, parentMissing := range []bool{false, true} {
		t.Run(fmt.Sprint(parentMissing), func(t *testing.T) {
			parent := t.TempDir()
			if parentMissing {
				parent = filepath.Join(parent, "absent")
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			m, err := Start(ctx, filepath.Join(parent, "config.toml"))
			var failure *Error
			if m != nil || !errors.As(err, &failure) || failure.Code != ERR_CONFIG_NOT_FOUND {
				t.Fatalf("Start: %v, %v", m, err)
			}
			if parentMissing {
				if _, err := os.Stat(parent); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("missing parent changed: %v", err)
				}
			}
		})
	}
}

func TestConfigWatchFailure(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.toml")
	writeWatchConfig(t, path, "A")
	w, target, err := openConfigWatch(path)
	if err != nil {
		t.Fatal(err)
	}
	failures := make(chan error)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	m, ready := attachConfigWatch(ctx, w, configWatch{target: target, events: w.Events, errors: failures}, func(context.Context) (Config, []Warning, error) { return loadFile(path) })
	t.Cleanup(func() { cancel(); waitWatchDone(t, m.Done()) })
	if err := <-ready; err != nil {
		t.Fatal(err)
	}
	select {
	case failures <- errors.New("private watch cause"):
	case <-ctx.Done():
		t.Fatal("watch failure not consumed")
	}
	s := awaitSnapshot(t, ctx, m, func(s Snapshot) bool { return s.Status.WatchFailure.Code != "" })
	if s.Config.Profile.SelectedID != "A" || s.Status.WatchFailure.Reason != "configuration watch failed" {
		t.Fatalf("watch failure state: %+v", s)
	}
	writeWatchConfig(t, path, "B")
	recovered := awaitWatchName(t, ctx, m, "B")
	if recovered.Status.WatchFailure != s.Status.WatchFailure {
		t.Fatal("config success cleared watch failure")
	}
}

func TestConfigWatchCancel(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.toml")
	writeWatchConfig(t, path, "A")
	w, target, err := openConfigWatch(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	m, ready := attachConfigWatch(ctx, w, configWatch{target: target, events: w.Events, errors: w.Errors}, func(context.Context) (Config, []Warning, error) { return loadFile(path) })
	if err := <-ready; err != nil {
		t.Fatal(err)
	}
	writeWatchConfig(t, path, "pending")
	awaitSnapshot(t, ctx, m, func(s Snapshot) bool { return s.Status.RequestGeneration > 1 })
	cancel()
	waitWatchDone(t, m.Done())
	if err := w.Add(filepath.Dir(path), fswatcher.Write); !errors.Is(err, fswatcher.ErrClosed) {
		t.Fatalf("watcher survives Done: %v", err)
	}
	for range m.Changes() {
	}
	if s := m.Snapshot(); s.Status.ConfigFailure != (Diagnostic{}) {
		t.Fatal("cancellation published failure")
	}
}

func TestConfigWatchEvents(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		input := make(chan fswatcher.Event)
		out := make(chan managerEvent)
		watch := configWatch{target: "/settings/config.toml", events: input}
		done := make(chan struct{})
		go func() { defer close(done); watch.run(ctx, out) }()
		started := time.Now()
		for range 3 {
			input <- fswatcher.Event{Name: watch.target, Op: fswatcher.Write}
			if got := <-out; got.kind != observeReload {
				t.Fatalf("event: %+v", got)
			}
			synctest.Wait()
			<-time.After(50 * time.Millisecond)
		}
		if got := <-out; got.kind != releaseReload {
			t.Fatalf("quiet event: %+v", got)
		}
		if elapsed := time.Since(started); elapsed != 300*time.Millisecond {
			t.Fatalf("burst quiet period: %v", elapsed)
		}
		synctest.Wait()
		select {
		case got := <-out:
			t.Fatalf("duplicate release: %+v", got)
		default:
		}
		cancel()
		<-done
	})
	for _, event := range []fswatcher.Event{{Name: "/settings/sibling", Op: fswatcher.Write}, {Name: "/settings/config.toml", Op: fswatcher.Chmod}} {
		if _, relevant := (configWatch{target: "/settings/config.toml"}).classify(event); relevant {
			t.Fatalf("unrelated event triggers reload: %+v", event)
		}
	}
}

func TestConfigWatchParent(t *testing.T) {
	t.Parallel()
	path, m, ctx, _ := watchFixture(t)
	if err := os.Rename(filepath.Dir(path), filepath.Dir(path)+"-moved"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(filepath.Dir(path) + "-moved"); err != nil {
			t.Error(err)
		}
	})
	s := awaitSnapshot(t, ctx, m, func(s Snapshot) bool { return s.Status.WatchFailure.Code != "" })
	if s.Config.Profile.SelectedID != "A" || s.Status.WatchFailure.Stage != "watch" {
		t.Fatalf("parent rename state: %+v", s)
	}
}

func TestConfigWatchSymlinkParent(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	real := filepath.Join(parent, "real")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(link, "config.toml")
	writeWatchConfig(t, path, "A")
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	m, err := Start(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); waitWatchDone(t, m.Done()) })
	writeWatchConfig(t, filepath.Join(real, "config.toml"), "B")
	awaitWatchName(t, ctx, m, "B")
}
