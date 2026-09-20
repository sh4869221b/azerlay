package input

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/sh4869221b/azerlay/internal/device"
)

type ManagedState string

const (
	ManagedConnected ManagedState = "connected"
	ManagedDegraded  ManagedState = "degraded"
	ManagedStopped   ManagedState = "stopped"
)

type ManagedSnapshot struct {
	Snapshot   *Snapshot
	State      ManagedState
	Diagnostic *device.Diagnostic
}

type Managed struct {
	mu      sync.Mutex
	session *Session
	latest  ManagedSnapshot
	cancel  context.CancelFunc
	done    chan struct{}
	err     error
}

type managedOps struct {
	discover func() (*device.Result, *device.Diagnostic)
	session  sessionOps
	retry    <-chan time.Time
	observe  func(ManagedSnapshot)
}

func StartManaged(ctx context.Context, target device.ReconnectTarget, generations Generations) (*Managed, error) {
	return startManaged(ctx, target, generations, managedOps{
		discover: device.Discover,
		session:  sessionOps{open: target.Open, clock: setMonotonicClock, axes: readAxisInfo},
	})
}

func startManaged(ctx context.Context, target device.ReconnectTarget, generations Generations, ops managedOps) (*Managed, error) {
	if !target.Valid() {
		d := device.ReconnectDiagnostic(errors.New("invalid reconnect target"))
		return nil, &d
	}
	ctx, cancel := context.WithCancel(ctx)
	m := &Managed{cancel: cancel, done: make(chan struct{}), latest: ManagedSnapshot{State: ManagedDegraded, Snapshot: &Snapshot{Generations: generations, reportingNode: -1}}}
	go m.run(ctx, target, generations, ops)
	return m, nil
}

func (m *Managed) Latest() ManagedSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := m.latest
	if m.session != nil {
		result.Snapshot = m.session.Latest()
		if !result.Snapshot.Connected {
			<-m.session.Done()
			err := m.session.Err()
			result.State = ManagedDegraded
			if err == nil || invalidManagedInput(err) {
				result.State = ManagedStopped
			}
			result.Diagnostic = managedDiagnostic(err)
		}
	}
	return copyManagedSnapshot(result)
}

func (m *Managed) Done() <-chan struct{} { return m.done }
func (m *Managed) Close() error          { m.cancel(); <-m.done; return m.err }

// Err returns a terminal or retained cleanup error after Done closes.
func (m *Managed) Err() error { return m.err }

func (m *Managed) run(ctx context.Context, target device.ReconnectTarget, generations Generations, ops managedOps) {
	var current *Session
	last := m.latest.Snapshot
	started := false
	defer func() {
		m.cancel()
		if current != nil {
			m.err = errors.Join(m.err, current.Close())
			last = current.Latest()
		}
		m.publish(nil, ManagedSnapshot{Snapshot: last, State: ManagedStopped, Diagnostic: managedDiagnostic(m.err)}, ops.observe)
		close(m.done)
	}()
	for {
		if ctx.Err() != nil {
			return
		}
		result, failure := ops.discover()
		if ctx.Err() != nil {
			return
		}
		var group device.Group
		var err error
		if failure != nil {
			err = failure
		} else {
			group, err = target.Select(result)
		}
		if err == nil {
			next := generations
			if started {
				next.Device++
			}
			current, err = startSession(ctx, group, next, ops.session)
			if ctx.Err() != nil {
				if err != nil && err != ctx.Err() {
					m.err = errors.Join(m.err, err)
				}
				return
			}
			if err == nil {
				generations, started = next, true
				m.publish(current, ManagedSnapshot{Snapshot: current.Latest(), State: ManagedConnected}, ops.observe)
				select {
				case <-ctx.Done():
					return
				case <-current.Done():
				}
				err = current.Err()
				last = current.Latest()
				if invalidManagedInput(err) {
					m.err = errors.Join(m.err, err)
					current = nil
					return
				}
				m.err = errors.Join(m.err, current.cleanupErr)
				current = nil
				if ctx.Err() != nil {
					return
				}
				if err == nil {
					err = &InputError{Code: ERR_INPUT_READ}
				}
			}
		}
		m.err = errors.Join(m.err, managedCleanupFailure(err))
		m.publish(nil, ManagedSnapshot{Snapshot: last, State: ManagedDegraded, Diagnostic: managedDiagnostic(err)}, ops.observe)
		if !waitManagedRetry(ctx, ops.retry) {
			return
		}
	}
}

func managedCleanupFailure(err error) error {
	switch err := err.(type) {
	case *sessionSetupError:
		return err.cleanup
	case *device.Diagnostic:
		if err.Stage == "close" {
			return err
		}
	case interface{ Unwrap() []error }:
		var cleanup error
		for _, cause := range err.Unwrap() {
			cleanup = errors.Join(cleanup, managedCleanupFailure(cause))
		}
		return cleanup
	case interface{ Unwrap() error }:
		return managedCleanupFailure(err.Unwrap())
	}
	return nil
}

func (m *Managed) publish(session *Session, state ManagedSnapshot, observe func(ManagedSnapshot)) {
	m.mu.Lock()
	m.session, m.latest = session, copyManagedSnapshot(state)
	m.mu.Unlock()
	if observe != nil {
		observe(copyManagedSnapshot(state))
	}
}

func waitManagedRetry(ctx context.Context, ticks <-chan time.Time) bool {
	if ticks == nil {
		timer := time.NewTimer(250 * time.Millisecond)
		defer timer.Stop()
		ticks = timer.C
	}
	select {
	case <-ctx.Done():
		return false
	case <-ticks:
		return true
	}
}

func invalidManagedInput(err error) bool {
	var inputErr *InputError
	return errors.As(err, &inputErr) && inputErr.Code == ERR_INPUT_EVENT
}

func managedDiagnostic(err error) *device.Diagnostic {
	if err == nil {
		return nil
	}
	var d *device.Diagnostic
	if errors.As(err, &d) {
		copy := *d
		return &copy
	}
	var inputErr *InputError
	if errors.As(err, &inputErr) && inputErr.cause == nil {
		d := &device.Diagnostic{Code: inputErr.Code, Severity: "error", Stage: "input"}
		if inputErr.Code == ERR_INPUT_EVENT {
			d.Summary = "Input events were invalid."
			d.Remediation = "Reconnect the selected device and restart input."
		} else {
			d.Summary = "Input could not be read."
			d.Remediation = "Check the selected device connection and access, then reconnect it."
		}
		return d
	}
	result := device.ReconnectDiagnostic(err)
	return &result
}

func copyManagedSnapshot(state ManagedSnapshot) ManagedSnapshot {
	snapshot := *state.Snapshot
	state.Snapshot = &snapshot
	if state.Diagnostic != nil {
		d := *state.Diagnostic
		if d.Target != nil {
			target := *d.Target
			d.Target = &target
		}
		state.Diagnostic = &d
	}
	return state
}
