package live

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/device"
	"github.com/sh4869221b/azerlay/internal/input"
)

type deviceSession interface {
	Latest() *input.Snapshot
	Changes() <-chan struct{}
	Done() <-chan struct{}
	Close() error
	Err() error
}
type managedInput interface {
	Latest() input.ManagedSnapshot
	Changes() <-chan struct{}
	Done() <-chan struct{}
	Close() error
}
type deviceOps struct {
	discover func() (*device.Result, *device.Diagnostic)
	start    func(context.Context, device.Group, input.Generations) (deviceSession, error)
	managed  func(context.Context, device.ReconnectTarget, input.Generations) (managedInput, error)
	retry    <-chan time.Time
}

// Device owns admission and joins the previous reader before replacing settings.
type Device struct {
	mu       sync.Mutex
	updateMu sync.Mutex
	ctx      context.Context
	settings config.Device
	ops      deviceOps
	latest   input.ManagedSnapshot
	changes  chan struct{}
	cancel   context.CancelFunc
	done     chan struct{}
	err      error
	closed   bool
}

func StartDevice(ctx context.Context, settings config.Device) *Device {
	return startDevice(ctx, settings, deviceOps{discover: device.Discover,
		start: func(ctx context.Context, group device.Group, generations input.Generations) (deviceSession, error) {
			return input.Start(ctx, group, generations)
		},
		managed: func(ctx context.Context, target device.ReconnectTarget, generations input.Generations) (managedInput, error) {
			return input.StartManaged(ctx, target, generations)
		},
	})
}
func startDevice(ctx context.Context, settings config.Device, ops deviceOps) *Device {
	d := &Device{ctx: ctx, settings: settings, ops: ops, changes: make(chan struct{}, 1), latest: input.ManagedSnapshot{State: input.ManagedDegraded, Snapshot: &input.Snapshot{Availability: input.Unavailable}}}
	d.begin()
	return d
}
func (d *Device) Latest() input.ManagedSnapshot {
	d.mu.Lock()
	defer d.mu.Unlock()
	state := d.latest
	snapshot := *state.Snapshot
	state.Snapshot = &snapshot
	return state
}
func (d *Device) Changes() <-chan struct{} { return d.changes }
func (d *Device) Update(settings config.Device) error {
	d.updateMu.Lock()
	defer d.updateMu.Unlock()
	if d.closed || settings == d.settings {
		return nil
	}
	d.cancel()
	<-d.done
	d.settings = settings
	err := d.err
	d.begin()
	return err
}
func (d *Device) Close() error {
	d.updateMu.Lock()
	defer d.updateMu.Unlock()
	d.closed = true
	d.cancel()
	<-d.done
	return d.err
}
func (d *Device) begin() {
	ctx, cancel := context.WithCancel(d.ctx)
	d.cancel, d.done = cancel, make(chan struct{})
	settings := d.settings
	baseline := d.Latest().Snapshot
	d.publish(input.ManagedSnapshot{State: input.ManagedDegraded, Snapshot: &input.Snapshot{Generations: baseline.Generations, Availability: input.Unavailable, EventCount: baseline.EventCount, InvalidationCount: baseline.InvalidationCount}})
	go func() {
		defer close(d.done)
		d.run(ctx, settings, baseline)
	}()
}
func (d *Device) publish(state input.ManagedSnapshot) {
	d.mu.Lock()
	d.latest = state
	d.mu.Unlock()
	select {
	case d.changes <- struct{}{}:
	default:
	}
}
func (d *Device) run(ctx context.Context, settings config.Device, baseline *input.Snapshot) {
	generations := baseline.Generations
	if generations.Device > 0 {
		generations.Device++
	} else {
		generations.Device = 1
	}
	unknown := &input.Snapshot{Generations: baseline.Generations, Availability: input.Unavailable, EventCount: baseline.EventCount, InvalidationCount: baseline.InvalidationCount}
	failure := func(err error) {
		d.publish(input.ManagedSnapshot{Snapshot: unknown, State: input.ManagedDegraded, Diagnostic: deviceFailure(err)})
	}
	if settings.Model != "cyborg-ii" || settings.Hand != "left" {
		failure(&device.Diagnostic{Code: device.ERR_DEVICE_UNSUPPORTED, Severity: "error", Stage: "selection", Summary: "The selected layout is unavailable."})
		return
	}
	for ctx.Err() == nil {
		result, diagnostic := d.ops.discover()
		var group device.Group
		var target device.ReconnectTarget
		var err error
		if diagnostic != nil {
			err = diagnostic
		} else {
			group, target, err = selectDevice(result, settings.Serial)
		}
		if err == nil && ctx.Err() == nil {
			if settings.AutoReconnect {
				var managed managedInput
				managed, err = d.ops.managed(ctx, target, generations)
				if err == nil {
					connected := false
					defer func() {
						d.err = errors.Join(d.err, managed.Close())
						state := managed.Latest()
						if !connected && state.Snapshot.Sequence == 0 {
							state.Snapshot.Generations = baseline.Generations
						}
						state.Snapshot.EventCount += baseline.EventCount
						state.Snapshot.InvalidationCount += baseline.InvalidationCount
						d.publish(state)
					}()
					for {
						state := managed.Latest()
						connected = connected || state.Snapshot.Connected || state.Snapshot.Sequence > 0
						if !connected {
							state.Snapshot.Generations = baseline.Generations
						}
						state.Snapshot.EventCount += baseline.EventCount
						state.Snapshot.InvalidationCount += baseline.InvalidationCount
						d.publish(state)
						select {
						case <-ctx.Done():
							return
						case <-managed.Done():
							return
						case <-managed.Changes():
						}
					}
				}
			} else {
				var session deviceSession
				session, err = d.ops.start(ctx, group, generations)
				if err == nil {
					defer func() {
						d.err = errors.Join(d.err, session.Close())
						snapshot := session.Latest()
						snapshot.EventCount += baseline.EventCount
						snapshot.InvalidationCount += baseline.InvalidationCount
						state := input.ManagedStopped
						if ctx.Err() == nil {
							state = input.ManagedDegraded
						}
						d.publish(input.ManagedSnapshot{Snapshot: snapshot, State: state, Diagnostic: deviceFailure(session.Err())})
					}()
					for {
						snapshot := session.Latest()
						snapshot.EventCount += baseline.EventCount
						snapshot.InvalidationCount += baseline.InvalidationCount
						state := input.ManagedConnected
						if !snapshot.Connected {
							state = input.ManagedDegraded
						}
						d.publish(input.ManagedSnapshot{Snapshot: snapshot, State: state})
						select {
						case <-ctx.Done():
							return
						case <-session.Done():
							snapshot = session.Latest()
							snapshot.EventCount += baseline.EventCount
							snapshot.InvalidationCount += baseline.InvalidationCount
							d.publish(input.ManagedSnapshot{Snapshot: snapshot, State: input.ManagedDegraded, Diagnostic: deviceFailure(session.Err())})
							return
						case <-session.Changes():
						}
					}
				}
			}
		}
		if ctx.Err() != nil {
			return
		}
		failure(err)
		if !settings.AutoReconnect {
			return
		}
		ticks := d.ops.retry
		var timer *time.Timer
		if ticks == nil {
			timer = time.NewTimer(250 * time.Millisecond)
			ticks = timer.C
		}
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-ticks:
		}
	}
}
func deviceFailure(err error) *device.Diagnostic {
	if err == nil {
		return nil
	}
	var diagnostic *device.Diagnostic
	if errors.As(err, &diagnostic) {
		copy := *diagnostic
		return &copy
	}
	var inputError *input.InputError
	if errors.As(err, &inputError) {
		return &device.Diagnostic{Code: inputError.Code, Severity: "error", Stage: "input", Summary: "Physical input is unavailable."}
	}
	copy := device.ReconnectDiagnostic(err)
	return &copy
}
func selectDevice(result *device.Result, serial string) (device.Group, device.ReconnectTarget, error) {
	var selected *device.Group
	if result != nil {
		for _, group := range result.Groups {
			if !group.Complete {
				continue
			}
			match := serial == ""
			for _, node := range result.Nodes {
				if node.USBParent != nil && *node.USBParent == group.USBParent && node.Serial != nil && *node.Serial == serial {
					match = true
				}
			}
			if !match {
				continue
			}
			if selected != nil {
				return device.Group{}, device.ReconnectTarget{}, &device.Diagnostic{Code: device.ERR_DEVICE_AMBIGUOUS, Severity: "error", Stage: "selection"}
			}
			copy := group
			selected = &copy
		}
	}
	if selected == nil {
		return device.Group{}, device.ReconnectTarget{}, &device.Diagnostic{Code: device.ERR_DEVICE_NOT_FOUND, Severity: "error", Stage: "selection"}
	}
	target, err := device.NewReconnectTarget(result, *selected)
	return *selected, target, err
}
