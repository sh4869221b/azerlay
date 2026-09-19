package config

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type candidate struct {
	config   Config
	warnings []Warning
	err      error
}

func configNamed(name string) Config {
	c := defaults()
	c.SchemaVersion = 1
	c.Profile.SelectedID = name
	return c
}

func startTestManager(t *testing.T, initial candidate) (*Manager, chan managerEvent, chan chan candidate, context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	events := make(chan managerEvent)
	loads := make(chan chan candidate)
	first := true
	m, ready := newManager(ctx, func(ctx context.Context) (Config, []Warning, error) {
		if first {
			first = false
			return initial.config, initial.warnings, initial.err
		}
		response := make(chan candidate)
		select {
		case loads <- response:
		case <-ctx.Done():
			return Config{}, nil, ctx.Err()
		}
		select {
		case result := <-response:
			return result.config, result.warnings, result.err
		case <-ctx.Done():
			return Config{}, nil, ctx.Err()
		}
	}, events)
	t.Cleanup(func() {
		cancel()
		select {
		case <-m.Done():
		case <-time.After(5 * time.Second):
			t.Error("manager did not stop")
		}
	})
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("initial load timed out")
	}
	return m, events, loads, ctx, cancel
}

func sendManagerEvent(t *testing.T, ctx context.Context, events chan<- managerEvent, event managerEvent) {
	t.Helper()
	select {
	case events <- event:
	case <-ctx.Done():
		t.Fatal("event not consumed")
	}
}

func nextLoad(t *testing.T, ctx context.Context, loads <-chan chan candidate) chan candidate {
	t.Helper()
	select {
	case response := <-loads:
		return response
	case <-ctx.Done():
		t.Fatal("load not started")
		return nil
	}
}

func completeLoad(t *testing.T, ctx context.Context, response chan<- candidate, result candidate) {
	t.Helper()
	select {
	case response <- result:
	case <-ctx.Done():
		t.Fatal("load not completed")
	}
}

func awaitSnapshot(t *testing.T, ctx context.Context, m *Manager, match func(Snapshot) bool) Snapshot {
	t.Helper()
	for {
		s := m.Snapshot()
		if match(s) {
			return s
		}
		select {
		case _, ok := <-m.Changes():
			if !ok {
				t.Fatal("manager stopped before expected snapshot")
			}
		case <-ctx.Done():
			t.Fatalf("snapshot timed out: %+v", s.Status)
		}
	}
}

func TestConfigReload(t *testing.T) {
	t.Parallel()
	m, events, loads, ctx, _ := startTestManager(t, candidate{config: configNamed("A")})
	if generation, err := m.RequestReload(ctx); err != nil || generation != 2 {
		t.Fatalf("reload acceptance: generation=%d err=%v", generation, err)
	}
	completeLoad(t, ctx, nextLoad(t, ctx, loads), candidate{err: &Error{Code: ERR_CONFIG_INVALID, Stage: "decode", Reason: "invalid TOML or field type", Cause: errors.New("private TOML secret")}})
	s := awaitSnapshot(t, ctx, m, func(s Snapshot) bool { return s.Status.ConfigFailure.Code != "" })
	if s.Config.Profile.SelectedID != "A" || s.Status.ConfigGeneration != 1 || s.Status.RequestGeneration != 2 || s.Status.ConfigFailure.Stage != "decode" {
		t.Fatalf("invalid reload lost good state: %+v", s)
	}
	if strings.Contains(fmt.Sprint(s.Status), "private") {
		t.Fatal("status exposed error cause")
	}
	watch := Diagnostic{Code: ERR_CONFIG_INVALID, Stage: "watch", Reason: "watch failed"}
	sendManagerEvent(t, ctx, events, managerEvent{kind: reportWatchFailure, failure: watch})
	if generation, err := m.RequestReload(ctx); err != nil || generation != 3 {
		t.Fatalf("recovery acceptance: generation=%d err=%v", generation, err)
	}
	completeLoad(t, ctx, nextLoad(t, ctx, loads), candidate{config: configNamed("C")})
	s = awaitSnapshot(t, ctx, m, func(s Snapshot) bool { return s.Status.ConfigGeneration == 3 })
	if s.Config.Profile.SelectedID != "C" || s.Status.ConfigFailure != (Diagnostic{}) || s.Status.WatchFailure != watch {
		t.Fatalf("recovery state: %+v", s)
	}
}

func TestConfigManualReload(t *testing.T) {
	t.Run("acceptance-and-coalescing", func(t *testing.T) {
		t.Parallel()
		for _, stale := range []candidate{{config: configNamed("obsolete")}, {err: &Error{Code: ERR_CONFIG_INVALID, Stage: "decode", Reason: "invalid TOML or field type"}}} {
			t.Run(fmt.Sprint(stale.err != nil), func(t *testing.T) {
				m, events, loads, ctx, _ := startTestManager(t, candidate{config: configNamed("A")})
				generation, err := m.RequestReload(ctx)
				if err != nil || generation != 2 {
					t.Fatalf("acceptance: generation=%d err=%v", generation, err)
				}
				active := nextLoad(t, ctx, loads)
				sendManagerEvent(t, ctx, events, managerEvent{kind: observeReload})
				for want := uint64(4); want <= 6; want++ {
					generation, err = m.RequestReload(ctx)
					if err != nil || generation != want {
						t.Fatalf("pending acceptance: generation=%d want=%d err=%v", generation, want, err)
					}
				}
				completeLoad(t, ctx, active, stale)
				latest := nextLoad(t, ctx, loads)
				if s := m.Snapshot(); s.Status.ConfigGeneration != 1 || s.Config.Profile.SelectedID != "A" || s.Status.ConfigFailure != (Diagnostic{}) {
					t.Fatalf("stale result published: %+v", s)
				}
				completeLoad(t, ctx, latest, candidate{config: configNamed("latest")})
				s := awaitSnapshot(t, ctx, m, func(s Snapshot) bool { return s.Status.ConfigGeneration == generation })
				if s.Config.Profile.SelectedID != "latest" {
					t.Fatal("latest manual reload did not publish")
				}
			})
		}
	})
	t.Run("real-file-and-watch", func(t *testing.T) {
		t.Parallel()
		path, m, ctx, _ := watchFixture(t)
		generation, err := m.RequestReload(ctx)
		if err != nil || generation != 2 {
			t.Fatalf("real-file acceptance: generation=%d err=%v", generation, err)
		}
		awaitSnapshot(t, ctx, m, func(s Snapshot) bool { return s.Status.ConfigGeneration == generation })
		writeWatchConfig(t, path, "watched")
		awaitWatchName(t, ctx, m, "watched")
	})
	t.Run("cancelled-and-stopped", func(t *testing.T) {
		t.Parallel()
		m, _, loads, ctx, cancel := startTestManager(t, candidate{config: configNamed("A")})
		requestCtx, cancelRequest := context.WithCancel(ctx)
		cancelRequest()
		if generation, err := m.RequestReload(requestCtx); generation != 0 || !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled acceptance: generation=%d err=%v", generation, err)
		}
		if generation, err := m.RequestReload(ctx); err != nil || generation != 2 {
			t.Fatalf("cancelled request changed generation: generation=%d err=%v", generation, err)
		}
		nextLoad(t, ctx, loads)
		cancel()
		waitWatchDone(t, m.Done())
		if generation, err := m.RequestReload(t.Context()); generation != 0 || !errors.Is(err, context.Canceled) {
			t.Fatalf("stopped acceptance: generation=%d err=%v", generation, err)
		}
	})
	t.Run("concurrent-cancellation", func(t *testing.T) {
		t.Parallel()
		m, _, _, ctx, cancel := startTestManager(t, candidate{config: configNamed("A")})
		requestCtx, cancelRequest := context.WithCancel(ctx)
		var requests sync.WaitGroup
		for range 32 {
			requests.Go(func() {
				generation, err := m.RequestReload(requestCtx)
				if err != nil && !errors.Is(err, context.Canceled) || err == nil && generation < 2 {
					t.Errorf("racing acceptance: generation=%d err=%v", generation, err)
				}
			})
		}
		cancelRequest()
		requests.Wait()
		if _, err := m.RequestReload(ctx); err != nil {
			t.Fatalf("cancelled caller blocked manager: %v", err)
		}
		cancel()
		waitWatchDone(t, m.Done())
	})
}

func TestConfigStaleReload(t *testing.T) {
	for _, stale := range []candidate{{config: configNamed("obsolete")}, {err: &Error{Code: ERR_CONFIG_INVALID, Stage: "decode", Reason: "invalid TOML or field type"}}} {
		t.Run("initial-"+fmt.Sprint(stale.err != nil), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			events := make(chan managerEvent)
			loads := make(chan chan candidate)
			m, ready := newManager(ctx, func(ctx context.Context) (Config, []Warning, error) {
				response := make(chan candidate)
				select {
				case loads <- response:
				case <-ctx.Done():
					return Config{}, nil, ctx.Err()
				}
				select {
				case result := <-response:
					return result.config, result.warnings, result.err
				case <-ctx.Done():
					return Config{}, nil, ctx.Err()
				}
			}, events)
			t.Cleanup(func() {
				cancel()
				select {
				case <-m.Done():
				case <-time.After(5 * time.Second):
					t.Error("initial loader did not stop")
				}
			})
			initial := nextLoad(t, ctx, loads)
			sendManagerEvent(t, ctx, events, managerEvent{kind: observeReload})
			awaitSnapshot(t, ctx, m, func(s Snapshot) bool { return s.Status.RequestGeneration == 2 })
			completeLoad(t, ctx, initial, stale)
			sendManagerEvent(t, ctx, events, managerEvent{kind: releaseReload})
			latest := nextLoad(t, ctx, loads)
			select {
			case err := <-ready:
				t.Fatalf("obsolete initial result completed startup: %v", err)
			default:
			}
			completeLoad(t, ctx, latest, candidate{config: configNamed("current")})
			select {
			case err := <-ready:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("current initial result did not complete startup")
			}
			if s := m.Snapshot(); s.Status.ConfigGeneration != 2 || s.Config.Profile.SelectedID != "current" {
				t.Fatalf("initial result: %+v", s)
			}
		})
		t.Run(fmt.Sprint(stale.err != nil), func(t *testing.T) {
			t.Parallel()
			m, events, loads, ctx, _ := startTestManager(t, candidate{config: configNamed("A")})
			sendManagerEvent(t, ctx, events, managerEvent{kind: requestReload})
			blocked := nextLoad(t, ctx, loads)
			for range 10 {
				sendManagerEvent(t, ctx, events, managerEvent{kind: observeReload})
			}
			awaitSnapshot(t, ctx, m, func(s Snapshot) bool { return s.Status.RequestGeneration == 12 })
			completeLoad(t, ctx, blocked, stale)
			sendManagerEvent(t, ctx, events, managerEvent{kind: releaseReload})
			latest := nextLoad(t, ctx, loads)
			s := m.Snapshot()
			if s.Config.Profile.SelectedID != "A" || s.Status.ConfigGeneration != 1 || s.Status.ConfigFailure != (Diagnostic{}) {
				t.Fatalf("stale result published: %+v", s)
			}
			completeLoad(t, ctx, latest, candidate{config: configNamed("latest")})
			awaitSnapshot(t, ctx, m, func(s Snapshot) bool { return s.Status.ConfigGeneration == 12 })
			if m.Snapshot().Config.Profile.SelectedID != "latest" {
				t.Fatal("pending latest request lost")
			}
		})
	}
}

func TestConfigSnapshotOwnership(t *testing.T) {
	t.Parallel()
	warnings := []Warning{{Code: WARN_CONFIG_UNKNOWN_KEY, Key: []string{"future", "setting"}}}
	m, _, _, _, _ := startTestManager(t, candidate{config: configNamed("A"), warnings: warnings})
	warnings[0].Key[0] = "source-mutated"
	s := m.Snapshot()
	if s.Warnings[0].Key[0] != "future" {
		t.Fatal("publication retained loader warning slice")
	}
	s.Warnings[0].Key[1] = "reader-mutated"
	s.Warnings[0].Code = "changed"
	s.Config.Profile.SelectedID = "changed"
	if got := m.Snapshot(); got.Warnings[0].Key[1] != "setting" || got.Warnings[0].Code != WARN_CONFIG_UNKNOWN_KEY || got.Config.Profile.SelectedID != "A" {
		t.Fatal("snapshot aliases manager state")
	}
}

func TestConfigConcurrentSnapshots(t *testing.T) {
	t.Parallel()
	m, events, loads, ctx, _ := startTestManager(t, candidate{config: configNamed("1")})
	var readers sync.WaitGroup
	stop := make(chan struct{})
	for range 4 {
		readers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				s := m.Snapshot()
				if s.Config.Profile.SelectedID != fmt.Sprint(s.Status.ConfigGeneration) || s.Status.ConfigGeneration > s.Status.RequestGeneration {
					t.Error("incoherent config/status snapshot")
					return
				}
			}
		})
	}
	defer func() { close(stop); readers.Wait() }()
	for generation := uint64(2); generation <= 20; generation++ {
		sendManagerEvent(t, ctx, events, managerEvent{kind: requestReload})
		completeLoad(t, ctx, nextLoad(t, ctx, loads), candidate{config: configNamed(fmt.Sprint(generation))})
		awaitSnapshot(t, ctx, m, func(s Snapshot) bool { return s.Status.ConfigGeneration == generation })
	}
	if cap(m.Changes()) != 1 {
		t.Fatal("notifications must have capacity one")
	}
}

func TestConfigManagerCancel(t *testing.T) {
	t.Parallel()
	m, events, loads, ctx, cancel := startTestManager(t, candidate{config: configNamed("A")})
	sendManagerEvent(t, ctx, events, managerEvent{kind: requestReload})
	nextLoad(t, ctx, loads)
	sendManagerEvent(t, ctx, events, managerEvent{kind: requestReload})
	before := awaitSnapshot(t, ctx, m, func(s Snapshot) bool { return s.Status.RequestGeneration == 3 })
	cancel()
	select {
	case <-m.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled loader not joined")
	}
	if !reflect.DeepEqual(before, m.Snapshot()) {
		t.Fatal("cancellation published a result")
	}
	for range m.Changes() {
	}
}
