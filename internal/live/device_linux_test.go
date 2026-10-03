package live

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/device"
	"github.com/sh4869221b/azerlay/internal/input"
)

func discoveryFixture(parent, serial, path string) *device.Result {
	return &device.Result{Groups: []device.Group{{USBParent: parent, Complete: true, HIDPaths: []string{path}}}, Nodes: []device.Node{{Path: path, USBParent: &parent, Serial: &serial, USBID: &device.USBID{Vendor: 0x16d0, Product: 0x12f7, Release: 0x111}, Interface: &device.Interface{Number: 4, Class: 3}, ReportDescriptor: &device.ReportDescriptor{UsagePage: 0xff01, Usage: 0x101, ReportBytes: 64}, Admission: "admitted"}}}
}

type ownerSession struct {
	mu       sync.Mutex
	snapshot *input.Snapshot
	changes  chan struct{}
	done     chan struct{}
	once     sync.Once
}

func newOwnerSession(g input.Generations) *ownerSession {
	return &ownerSession{snapshot: &input.Snapshot{Connected: true, Generations: g, Availability: input.Unconfirmed}, changes: make(chan struct{}, 1), done: make(chan struct{})}
}
func (s *ownerSession) Latest() *input.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := *s.snapshot
	return &copy
}
func (s *ownerSession) Changes() <-chan struct{} { return s.changes }
func (s *ownerSession) Done() <-chan struct{}    { return s.done }
func (s *ownerSession) Err() error               { return nil }
func (s *ownerSession) Close() error {
	s.once.Do(func() {
		s.mu.Lock()
		s.snapshot = &input.Snapshot{Generations: s.snapshot.Generations, Availability: input.Unavailable, EventCount: 3}
		s.mu.Unlock()
		close(s.done)
	})
	return nil
}
func awaitDevice(t *testing.T, d *Device, predicate func(input.ManagedSnapshot) bool) input.ManagedSnapshot {
	t.Helper()
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		state := d.Latest()
		if predicate(state) {
			return state
		}
		select {
		case <-d.Changes():
		case <-timer.C:
			t.Fatal("device owner blocked")
			return state
		}
	}
}

func TestLiveDeviceLifecycle(t *testing.T) {
	t.Run("replace-and-join", func(t *testing.T) {
		settings := config.Device{Model: "cyborg-ii", Hand: "left", Serial: "first"}
		var first *ownerSession
		opened := make(chan *ownerSession, 2)
		result := discoveryFixture("/usb/first", "first", "/hidraw1")
		var resultMu sync.Mutex
		d := startDevice(context.Background(), settings, deviceOps{discover: func() (*device.Result, *device.Diagnostic) {
			resultMu.Lock()
			defer resultMu.Unlock()
			return result, nil
		}, start: func(_ context.Context, _ device.Group, g input.Generations) (deviceSession, error) {
			if first != nil {
				select {
				case <-first.Done():
				default:
					t.Error("old session not joined")
				}
			}
			s := newOwnerSession(g)
			opened <- s
			return s, nil
		}})
		t.Cleanup(func() { d.Close() })
		first = <-opened
		state := awaitDevice(t, d, func(s input.ManagedSnapshot) bool { return s.Snapshot.Connected })
		if state.Snapshot.Generations.Device != 1 {
			t.Fatal(state)
		}
		resultMu.Lock()
		result = discoveryFixture("/usb/second", "second", "/hidraw9")
		resultMu.Unlock()
		settings.Serial = "second"
		if err := d.Update(settings); err != nil {
			t.Fatal(err)
		}
		second := <-opened
		state = awaitDevice(t, d, func(s input.ManagedSnapshot) bool { return s.Snapshot.Connected && s.Snapshot.Generations.Device == 2 })
		if state.Snapshot.EventCount != 3 || state.Snapshot.Availability != input.Unconfirmed {
			t.Fatal(state)
		}
		if err := second.Close(); err != nil {
			t.Fatal(err)
		}
		awaitDevice(t, d, func(s input.ManagedSnapshot) bool { return !s.Snapshot.Connected })
		if err := d.Update(settings); err != nil {
			t.Fatal(err)
		}
		if len(opened) != 0 {
			t.Fatal("auto_reconnect false reopened")
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
		if d.Latest().Snapshot.Connected {
			t.Fatal("close retained connected snapshot")
		}
	})
	for _, tc := range []struct {
		name      string
		hand      string
		discovery *device.Result
		openErr   error
		code      string
	}{
		{"absent", "left", &device.Result{}, nil, device.ERR_DEVICE_NOT_FOUND},
		{"permission", "left", discoveryFixture("/usb/a", "a", "/hidraw1"), syscall.EACCES, device.ERR_DEVICE_PERMISSION},
		{"right-layout", "right", discoveryFixture("/usb/a", "a", "/hidraw1"), nil, device.ERR_DEVICE_UNSUPPORTED},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var attempts atomic.Int32
			d := startDevice(context.Background(), config.Device{Model: "cyborg-ii", Hand: tc.hand}, deviceOps{discover: func() (*device.Result, *device.Diagnostic) { attempts.Add(1); return tc.discovery, nil }, start: func(context.Context, device.Group, input.Generations) (deviceSession, error) { return nil, tc.openErr }})
			state := awaitDevice(t, d, func(s input.ManagedSnapshot) bool { return s.Diagnostic != nil })
			if state.Diagnostic.Code != tc.code || state.Snapshot.Connected || state.Snapshot.Generations.Device != 0 {
				t.Fatal(state)
			}
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
			want := int32(1)
			if tc.hand == "right" {
				want = 0
			}
			if attempts.Load() != want {
				t.Fatal("unexpected retry/open")
			}
		})
	}
	t.Run("admission-retry-and-ambiguity", func(t *testing.T) {
		one := discoveryFixture("/usb/a", "a", "/hidraw1")
		two := discoveryFixture("/usb/b", "b", "/hidraw2")
		both := &device.Result{Groups: append(one.Groups, two.Groups...), Nodes: append(one.Nodes, two.Nodes...)}
		if _, _, err := selectDevice(both, ""); err == nil {
			t.Fatal("ambiguous device selected")
		}
		group, target, err := selectDevice(both, "b")
		if err != nil || group.USBParent != "/usb/b" || !target.Valid() {
			t.Fatal(group, err)
		}
		renumbered := discoveryFixture("/usb/new", "b", "/hidraw9")
		if group, err := target.Select(renumbered); err != nil || group.HIDPaths[0] != "/hidraw9" {
			t.Fatal(group, err)
		}
		var attempts atomic.Int32
		ticks := make(chan time.Time)
		called := make(chan struct{})
		d := startDevice(context.Background(), config.Device{Model: "cyborg-ii", Hand: "left", AutoReconnect: true}, deviceOps{discover: func() (*device.Result, *device.Diagnostic) {
			if attempts.Add(1) == 1 {
				return &device.Result{}, nil
			}
			return one, nil
		}, managed: func(context.Context, device.ReconnectTarget, input.Generations) (managedInput, error) {
			close(called)
			return nil, errors.New("synthetic open failure")
		}, retry: ticks})
		awaitDevice(t, d, func(s input.ManagedSnapshot) bool { return s.Diagnostic != nil })
		select {
		case ticks <- time.Time{}:
		case <-time.After(3 * time.Second):
			t.Fatal("retry blocked")
		}
		select {
		case <-called:
		case <-time.After(3 * time.Second):
			t.Fatal("managed start not called")
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
		if attempts.Load() != 2 {
			t.Fatal(attempts.Load())
		}
	})
}
