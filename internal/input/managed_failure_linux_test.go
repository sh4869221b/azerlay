package input

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/device"
)

func TestManagedAmbiguousTarget(t *testing.T) {
	for _, absent := range []bool{false, true} {
		t.Run(map[bool]string{false: "duplicate", true: "absent"}[absent], func(t *testing.T) {
			t.Parallel()
			r := managedDiscovery("/synthetic/event1")
			target, err := device.NewReconnectTarget(r, r.Groups[0])
			if err != nil {
				t.Fatal(err)
			}
			code := device.ERR_DEVICE_AMBIGUOUS
			if absent {
				r = &device.Result{}
				code = device.ERR_DEVICE_NOT_FOUND
			} else {
				duplicate := managedDiscovery("/synthetic/event4")
				parent := "/synthetic/usb/other"
				duplicate.Groups[0].USBParent = parent
				duplicate.Nodes[0].USBParent = &parent
				r.Groups = append(r.Groups, duplicate.Groups...)
				r.Nodes = append(r.Nodes, duplicate.Nodes...)
			}
			states := make(chan ManagedSnapshot, 4)
			opens := 0
			m, err := startManaged(context.Background(), target, Generations{}, managedOps{
				discover: func() (*device.Result, *device.Diagnostic) { return r, nil },
				session:  sessionOps{open: func(device.Group) ([]device.OpenedNode, error) { opens++; return nil, syscall.EACCES }},
				retry:    make(chan time.Time), observe: func(s ManagedSnapshot) { states <- s },
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = m.Close() })
			state := awaitManaged(t, states, ManagedDegraded)
			if state.Diagnostic == nil || state.Diagnostic.Code != code {
				t.Fatalf("selection diagnostic=%+v", state)
			}
			if err := m.Close(); err != nil {
				t.Fatal(err)
			}
			awaitManaged(t, states, ManagedStopped)
			if opens != 0 {
				t.Fatalf("unselected descriptors opened %d times", opens)
			}
		})
	}
}

func TestManagedCancellation(t *testing.T) {
	for _, phase := range []string{"discover", "open", "clock", "axes"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			r := managedDiscovery("/synthetic/event1")
			target, err := device.NewReconnectTarget(r, r.Groups[0])
			if err != nil {
				t.Fatal(err)
			}
			nodes, _ := pipeNodes(t, 1)
			nodes[0].Node = r.Nodes[0]
			entered, exited := make(chan struct{}), make(chan struct{})
			release := make(chan struct{}, 1)
			block := func() { close(entered); <-release; close(exited) }
			states := make(chan ManagedSnapshot, 4)
			ops := managedPipeOps(nodes, make(chan integrationObservation, 4))
			ops.open = func(device.Group) ([]device.OpenedNode, error) {
				if phase == "open" {
					block()
				}
				return nodes, nil
			}
			ops.clock = func(*os.File) error {
				if phase == "clock" {
					block()
				}
				return nil
			}
			ops.axes = func(*os.File, []int) (map[uint16]AxisInfo, error) {
				if phase == "axes" {
					block()
				}
				return nil, nil
			}
			m, err := startManaged(ctx, target, Generations{Device: 7, Profile: 9}, managedOps{
				discover: func() (*device.Result, *device.Diagnostic) {
					if phase == "discover" {
						block()
					}
					return r, nil
				},
				session: ops, retry: make(chan time.Time), observe: func(s ManagedSnapshot) { states <- s },
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				cancel()
				close(release)
				_ = m.Close()
			})
			awaitRecovery(t, entered)
			cancel()
			// Parent Done closes before cancellation reaches the manager's child context.
			// Resume setup only after cancel returns and propagation is complete.
			release <- struct{}{}
			awaitRecovery(t, m.Done())
			awaitRecovery(t, exited)
			if err := m.Close(); err != nil {
				t.Fatal(err)
			}
			stopped := awaitManaged(t, states, ManagedStopped)
			if stopped.Diagnostic != nil || stopped.Snapshot.Generations != (Generations{Device: 7, Profile: 9}) {
				t.Fatalf("canceled manager=%+v", stopped)
			}
			if phase == "discover" {
				if _, err := nodes[0].File.Stat(); err != nil {
					t.Fatal("manager touched unacquired descriptor")
				}
			} else {
				assertFilesClosed(t, nodes)
			}
		})
	}
}

func TestManagedReplacementOwnership(t *testing.T) {
	t.Parallel()
	r := managedDiscovery("/synthetic/event1", "/synthetic/event2")
	target, err := device.NewReconnectTarget(r, r.Groups[0])
	if err != nil {
		t.Fatal(err)
	}
	failed, _ := pipeNodes(t, 2)
	next, _ := pipeNodes(t, 2)
	states := make(chan ManagedSnapshot, 4)
	ticks := make(chan time.Time)
	ops := managedPipeOps(next, make(chan integrationObservation, 4))
	attempts := 0
	ops.open = func(device.Group) ([]device.OpenedNode, error) {
		attempts++
		if attempts == 1 {
			return failed, nil
		}
		return next, nil
	}
	ops.clock = func(file *os.File) error {
		if attempts == 1 {
			if err := file.Close(); err != nil {
				return err
			}
			return syscall.EACCES
		}
		return nil
	}
	m, err := startManaged(context.Background(), target, Generations{Device: 7, Profile: 9}, managedOps{discover: func() (*device.Result, *device.Diagnostic) { return r, nil }, session: ops, retry: ticks, observe: func(s ManagedSnapshot) { states <- s }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	degraded := awaitManaged(t, states, ManagedDegraded)
	if degraded.Diagnostic.Code != device.ERR_DEVICE_PERMISSION {
		t.Fatalf("setup failure=%+v", degraded)
	}
	assertFilesClosed(t, failed)
	tickManaged(t, ticks)
	connected := awaitManaged(t, states, ManagedConnected)
	if connected.Snapshot.Generations.Device != 7 {
		t.Fatal("failed setup advanced generation")
	}
	if err := m.Close(); err == nil || !errors.Is(err, syscall.ENODEV) {
		t.Fatalf("rollback close error lost after success: %v", err)
	}
	awaitManaged(t, states, ManagedStopped)
	assertFilesClosed(t, next)
}

func TestManagedInvalidInputStops(t *testing.T) {
	t.Parallel()
	r := managedDiscovery("/synthetic/event1")
	target, err := device.NewReconnectTarget(r, r.Groups[0])
	if err != nil {
		t.Fatal(err)
	}
	nodes, writers := pipeNodes(t, 1)
	states := make(chan ManagedSnapshot, 4)
	ticks := make(chan time.Time)
	m, err := startManaged(context.Background(), target, Generations{}, managedOps{discover: func() (*device.Result, *device.Diagnostic) { return r, nil }, session: managedPipeOps(nodes, make(chan integrationObservation, 4)), retry: ticks, observe: func(s ManagedSnapshot) { states <- s }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	awaitManaged(t, states, ManagedConnected)
	if _, err := writers[0].Write(nativeEvents(Event{Type: EV_KEY, Code: 30, Value: 3})); err != nil {
		t.Fatal(err)
	}
	stopped := awaitManaged(t, states, ManagedStopped)
	awaitRecovery(t, m.Done())
	if stopped.Diagnostic.Code != ERR_INPUT_EVENT || m.Err() == nil || m.Err().Error() != ERR_INPUT_EVENT {
		t.Fatalf("invalid input terminal=%+v, %v", stopped, m.Err())
	}
	assertFilesClosed(t, nodes)
	select {
	case ticks <- time.Time{}:
		t.Fatal("invalid input retried")
	default:
	}
}

func TestManagedInvalidTarget(t *testing.T) {
	m, err := StartManaged(context.Background(), device.ReconnectTarget{}, Generations{})
	if m != nil || err == nil || err.Error() != device.ERR_DEVICE_METADATA {
		t.Fatalf("invalid target=%v, %v", m, err)
	}
}

func TestManagedLatestTerminal(t *testing.T) {
	t.Parallel()
	r := managedDiscovery("/synthetic/event1")
	target, err := device.NewReconnectTarget(r, r.Groups[0])
	if err != nil {
		t.Fatal(err)
	}
	nodes, writers := pipeNodes(t, 1)
	states := make(chan ManagedSnapshot, 4)
	resume := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m, err := startManaged(ctx, target, Generations{}, managedOps{
		discover: func() (*device.Result, *device.Diagnostic) { return r, nil },
		session:  managedPipeOps(nodes, make(chan integrationObservation, 4)), retry: make(chan time.Time),
		observe: func(state ManagedSnapshot) {
			states <- state
			if state.State == ManagedConnected {
				select {
				case <-resume:
				case <-ctx.Done():
				}
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); _ = m.Close() })
	awaitManaged(t, states, ManagedConnected)
	session := activeManagedSession(m)
	if err := writers[0].Close(); err != nil {
		t.Fatal(err)
	}
	awaitSession(t, session)
	latest := m.Latest()
	if latest.State != ManagedDegraded || latest.Snapshot.Connected || latest.Diagnostic.Code != ERR_INPUT_READ {
		t.Fatalf("terminal session appeared connected: %+v", latest)
	}
	assertFilesClosed(t, nodes)
	close(resume)
	awaitManaged(t, states, ManagedDegraded)
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	awaitManaged(t, states, ManagedStopped)
}

func TestManagedOpenCleanupFailure(t *testing.T) {
	t.Parallel()
	r := managedDiscovery("/synthetic/event1")
	target, err := device.NewReconnectTarget(r, r.Groups[0])
	if err != nil {
		t.Fatal(err)
	}
	failed, _ := pipeNodes(t, 1)
	next, _ := pipeNodes(t, 1)
	states := make(chan ManagedSnapshot, 4)
	ticks := make(chan time.Time)
	ops := managedPipeOps(next, make(chan integrationObservation, 4))
	attempts := 0
	ops.open = func(device.Group) ([]device.OpenedNode, error) {
		attempts++
		if attempts > 1 {
			return next, nil
		}
		if err := failed[0].File.Close(); err != nil {
			return nil, err
		}
		closeErr := failed[0].File.Close()
		if closeErr == nil {
			return nil, &InputError{Code: ERR_INPUT_EVENT}
		}
		permission := device.ReconnectDiagnostic(syscall.EACCES)
		cleanup := device.ReconnectDiagnostic(closeErr)
		cleanup.Stage = "close"
		return nil, errors.Join(&permission, &cleanup)
	}
	m, err := startManaged(context.Background(), target, Generations{}, managedOps{discover: func() (*device.Result, *device.Diagnostic) { return r, nil }, session: ops, retry: ticks, observe: func(s ManagedSnapshot) { states <- s }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	awaitManaged(t, states, ManagedDegraded)
	tickManaged(t, ticks)
	awaitManaged(t, states, ManagedConnected)
	var diagnostic *device.Diagnostic
	if err := m.Close(); !errors.As(err, &diagnostic) || diagnostic.Stage != "close" {
		t.Fatalf("open rollback cleanup lost: %v", err)
	}
	assertFilesClosed(t, next)
}
