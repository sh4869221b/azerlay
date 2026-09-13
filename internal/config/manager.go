package config

import (
	"context"
	"errors"
	"slices"
	"sync"
)

// Diagnostic contains safe status details, without the underlying error cause.
type Diagnostic struct{ Code, Stage, Reason string }

type Status struct {
	RequestGeneration, ConfigGeneration uint64
	ConfigFailure, WatchFailure         Diagnostic
}

type Snapshot struct {
	Config   Config
	Warnings []Warning
	Status   Status
}

type Manager struct {
	mu       sync.Mutex
	snapshot Snapshot
	changes  chan struct{}
	done     chan struct{}
}

type managerEventKind uint8

const (
	requestReload managerEventKind = iota
	observeReload
	releaseReload
	reportWatchFailure
)

type managerEvent struct {
	kind    managerEventKind
	failure Diagnostic
}
type configLoader func(context.Context) (Config, []Warning, error)

func newManager(ctx context.Context, load configLoader, events <-chan managerEvent) (*Manager, <-chan error) {
	m := &Manager{changes: make(chan struct{}, 1), done: make(chan struct{})}
	ready := make(chan error, 1)
	go func() {
		m.run(ctx, load, events, ready)
		close(m.done)
	}()
	return m, ready
}

func (m *Manager) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := m.snapshot
	snapshot.Warnings = cloneWarnings(snapshot.Warnings)
	return snapshot
}
func (m *Manager) Changes() <-chan struct{} { return m.changes }
func (m *Manager) Done() <-chan struct{}    { return m.done }

type loadResult struct {
	generation uint64
	config     Config
	warnings   []Warning
	err        error
}

func (m *Manager) run(ctx context.Context, load configLoader, events <-chan managerEvent, ready chan<- error) {
	ctx, cancel := context.WithCancel(ctx)
	jobs := make(chan uint64)
	results := make(chan loadResult)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		for {
			select {
			case <-ctx.Done():
				return
			case generation := <-jobs:
				config, warnings, err := load(ctx)
				result := loadResult{generation: generation, config: config, warnings: cloneWarnings(warnings), err: err}
				select {
				case results <- result:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	initialized := false
	defer func() {
		cancel()
		<-workerDone
		close(ready)
		close(m.changes)
	}()
	state := Snapshot{Status: Status{RequestGeneration: 1}}
	pending, eligible, active := true, true, false
	for {
		if ctx.Err() != nil {
			if !initialized {
				ready <- ctx.Err()
			}
			return
		}
		var start chan<- uint64
		if pending && eligible && !active {
			start = jobs
		}
		select {
		case <-ctx.Done():
			if !initialized {
				ready <- ctx.Err()
			}
			return
		case start <- state.Status.RequestGeneration:
			pending, active = false, true
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			if ctx.Err() != nil {
				continue
			}
			switch event.kind {
			case requestReload, observeReload:
				state.Status.RequestGeneration++
				pending = true
				eligible = event.kind == requestReload
			case releaseReload:
				eligible = true
			case reportWatchFailure:
				state.Status.WatchFailure = event.failure
			}
			m.publish(state)
		case result := <-results:
			active = false
			if ctx.Err() != nil || result.generation != state.Status.RequestGeneration {
				continue
			}
			if result.err != nil {
				if !initialized {
					ready <- result.err
					return
				}
				state.Status.ConfigFailure = configDiagnostic(result.err)
			} else {
				state.Config = result.config
				state.Warnings = result.warnings
				state.Status.ConfigGeneration = result.generation
				state.Status.ConfigFailure = Diagnostic{}
			}
			m.publish(state)
			if !initialized {
				initialized = true
				ready <- nil
			}
		}
	}
}

func (m *Manager) publish(snapshot Snapshot) {
	m.mu.Lock()
	m.snapshot = snapshot
	m.mu.Unlock()
	select {
	case m.changes <- struct{}{}:
	default:
	}
}

func cloneWarnings(warnings []Warning) []Warning {
	owned := slices.Clone(warnings)
	for i := range owned {
		owned[i].Key = slices.Clone(owned[i].Key)
	}
	return owned
}

func configDiagnostic(err error) Diagnostic {
	var configError *Error
	if errors.As(err, &configError) {
		return Diagnostic{Code: configError.Code, Stage: configError.Stage, Reason: configError.Reason}
	}
	return Diagnostic{Code: ERR_CONFIG_INVALID, Stage: "read", Reason: "configuration load failed"}
}
