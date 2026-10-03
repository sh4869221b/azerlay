package live

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/control"
	"github.com/sh4869221b/azerlay/internal/input"
)

type coordinatorConfig struct {
	value   atomic.Pointer[config.Snapshot]
	changes chan struct{}
}

func (s *coordinatorConfig) Snapshot() config.Snapshot { return *s.value.Load() }
func (s *coordinatorConfig) Changes() <-chan struct{}  { return s.changes }

type coordinatorProfile struct {
	value   atomic.Pointer[control.ProfileSnapshot]
	changes chan struct{}
	once    sync.Once
	barrier func()
}

func (s *coordinatorProfile) ProfileSnapshot() *control.ProfileSnapshot {
	value := s.value.Load()
	if s.barrier != nil {
		s.once.Do(s.barrier)
	}
	return value
}
func (s *coordinatorProfile) Changes() <-chan struct{}           { return s.changes }
func (s *coordinatorProfile) VisibilityChanges() <-chan struct{} { return nil }
func (s *coordinatorProfile) RequestedVisible() bool             { return false }

type coordinatorInput struct {
	value     atomic.Pointer[input.Snapshot]
	changes   chan struct{}
	closed    chan struct{}
	sequence  atomic.Uint64
	advancing bool
	barrier   func()
	once      sync.Once
}

func (s *coordinatorInput) Latest() input.ManagedSnapshot {
	value := *s.value.Load()
	if s.barrier != nil {
		s.once.Do(s.barrier)
	}
	if s.advancing {
		value.Sequence = s.sequence.Add(1)
	}
	return input.ManagedSnapshot{Snapshot: &value}
}
func (s *coordinatorInput) Changes() <-chan struct{}   { return s.changes }
func (s *coordinatorInput) Update(config.Device) error { return nil }
func (s *coordinatorInput) Close() error               { close(s.closed); return nil }

func coordinatorFixture(t *testing.T) (Sources, *coordinatorConfig, *coordinatorProfile, *coordinatorInput) {
	t.Helper()
	cfg := &coordinatorConfig{changes: make(chan struct{}, 1)}
	cfg.value.Store(&config.Snapshot{Config: config.Config{Device: config.Device{Model: "cyborg-ii", Hand: "left"}, Profile: config.Profile{Game: "unknown"}}, Status: config.Status{ConfigGeneration: 1}})
	selected := &coordinatorProfile{changes: make(chan struct{}, 1)}
	selected.value.Store(assemblyFixture(t).Profile)
	raw := &coordinatorInput{changes: make(chan struct{}, 1), closed: make(chan struct{})}
	raw.value.Store(&input.Snapshot{Connected: true, Generations: input.Generations{Device: 1}})
	return Sources{Config: cfg, Profile: selected, Input: raw}, cfg, selected, raw
}

func awaitView(t *testing.T, c *Coordinator, predicate func(*View) bool) *View {
	t.Helper()
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		if view := c.Latest(); view != nil && predicate(view) {
			return view
		}
		select {
		case <-c.Changes():
		case <-timer.C:
			t.Fatal("coordinator did not publish")
			return nil
		}
	}
}

func TestGenerations(t *testing.T) {
	for _, component := range []string{"config", "profile", "device"} {
		t.Run(component, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			sources, cfg, selected, raw := coordinatorFixture(t)
			entered, release := make(chan struct{}), make(chan struct{})
			if component == "device" {
				raw.barrier = func() { close(entered); <-release }
			} else {
				selected.barrier = func() { close(entered); <-release }
			}
			c := Start(t.Context(), sources)
			<-entered
			switch component {
			case "config":
				next := cfg.Snapshot()
				next.Status.ConfigGeneration++
				cfg.value.Store(&next)
			case "profile":
				next := *selected.value.Load()
				next.Generation++
				selected.value.Store(&next)
			case "device":
				next := *raw.value.Load()
				next.Generations.Device++
				raw.value.Store(&next)
			}
			close(release)
			view := awaitView(t, c, func(v *View) bool { return true })
			if view.Generations.Config != cfg.Snapshot().Status.ConfigGeneration || view.Generations.Profile != selected.value.Load().Generation || view.Generations.Device != raw.value.Load().Generations.Device {
				t.Fatalf("stale result published: %+v", view.Generations)
			}
			if err := c.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-raw.closed:
			default:
				t.Fatal("input not joined")
			}
		})
	}
	t.Run("reports-do-not-require-silence", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		sources, _, _, raw := coordinatorFixture(t)
		raw.advancing = true
		c := Start(t.Context(), sources)
		defer c.Close()
		awaitView(t, c, func(v *View) bool { return v.Input.Snapshot.Sequence > 0 })
	})
}

func TestAssemblyCatalogReload(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	dir := filepath.Join(root, "azerlay", "games")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "synthetic.toml")
	if err := os.WriteFile(path, []byte("schema_version=1\nid='synthetic'\nname='Synthetic'\nlocale='en'\n[controls]\n'input:4:single'='Synthetic label'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	sources, cfg, _, _ := coordinatorFixture(t)
	next := cfg.Snapshot()
	next.Config.Profile.Game = "synthetic"
	cfg.value.Store(&next)
	c := Start(t.Context(), sources)
	defer c.Close()
	before := awaitView(t, c, func(v *View) bool { return true })
	if len(before.Diagnostics) != 0 {
		t.Fatalf("initial catalog: %+v", before.Diagnostics)
	}
	if err := os.WriteFile(path, []byte("invalid=["), 0600); err != nil {
		t.Fatal(err)
	}
	next.Status.ConfigGeneration++
	cfg.value.Store(&next)
	cfg.changes <- struct{}{}
	after := awaitView(t, c, func(v *View) bool { return v.Generations.Config == 2 })
	if len(after.Diagnostics) != 1 || after.Diagnostics[0].Code != "GAME_LABELS_UNAVAILABLE" || after.Render != before.Render {
		t.Fatal("failed label reload lost last-good render or diagnostic")
	}
}
