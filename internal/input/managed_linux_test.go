package input

import (
	"context"
	"errors"
	"github.com/sh4869221b/azerlay/internal/device"
	"os"
	"syscall"
	"testing"
	"time"
)

func managedDiscovery(path string) *device.Result {
	parent, serial := "/synthetic/usb", "synthetic"
	return &device.Result{Groups: []device.Group{{USBParent: parent, Complete: true, HIDPaths: []string{path}}}, Nodes: []device.Node{{Path: path, USBParent: &parent, USBID: &device.USBID{Vendor: 0x16d0, Product: 0x12f7, Release: 0x111}, Interface: &device.Interface{Number: 4, Class: 3}, ReportDescriptor: &device.ReportDescriptor{UsagePage: 0xff01, Usage: 0x101, ReportBytes: 64}, Roles: []string{"physical-buttons"}, Admission: "admitted", Serial: &serial}}}
}
func awaitManaged(t *testing.T, c <-chan ManagedSnapshot, want ManagedState) ManagedSnapshot {
	t.Helper()
	select {
	case s := <-c:
		if s.State != want {
			t.Fatalf("state=%+v want=%s", s, want)
		}
		return s
	case <-time.After(3 * time.Second):
		t.Fatal("manager blocked")
		return ManagedSnapshot{}
	}
}
func TestPhysicalManagedReconnect(t *testing.T) {
	r := managedDiscovery("/synthetic/hidraw1")
	target, err := device.NewReconnectTarget(r, r.Groups[0])
	if err != nil {
		t.Fatal(err)
	}
	first, w1 := pipeNode(t)
	second, w2 := pipeNode(t)
	states := make(chan ManagedSnapshot, 8)
	events := make(chan *Snapshot, 8)
	ticks := make(chan time.Time)
	attempts := 0
	var oldSession *Session
	m, err := startManaged(context.Background(), target, Generations{Device: 7, Profile: 9}, managedOps{discover: func() (*device.Result, *device.Diagnostic) {
		if attempts > 0 {
			return managedDiscovery("/synthetic/hidraw9"), nil
		}
		return r, nil
	}, session: sessionOps{open: func(g device.Group) ([]device.OpenedNode, error) {
		attempts++
		if attempts == 1 {
			if g.HIDPaths[0] != "/synthetic/hidraw1" {
				t.Error("initial path")
			}
			return first, nil
		}
		if g.HIDPaths[0] != "/synthetic/hidraw9" {
			t.Error("reconnect path")
		}
		select {
		case <-oldSession.Done():
		default:
			t.Error("old session still running at replacement open")
		}
		if _, err := first[0].File.Stat(); !errors.Is(err, os.ErrClosed) {
			t.Error("old fd still open at replacement open")
		}
		if attempts == 2 {
			return nil, syscall.EACCES
		}
		return second, nil
	}, observe: func(s *Snapshot) { events <- s }}, retry: ticks, observe: func(s ManagedSnapshot) { states <- s }})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	initial := awaitManaged(t, states, ManagedConnected)
	if initial.Snapshot.Generations != (Generations{7, 9}) {
		t.Fatal(initial)
	}
	m.mu.Lock()
	oldSession = m.session
	m.mu.Unlock()
	if _, err := w1.Write(physicalBytes(4, 1, 1)); err != nil {
		t.Fatal(err)
	}
	old := awaitSnapshot(t, events)
	if old.Generations != (Generations{7, 9}) {
		t.Fatal(old)
	}
	if err := w1.Close(); err != nil {
		t.Fatal(err)
	}
	degraded := awaitManaged(t, states, ManagedDegraded)
	if degraded.Snapshot.Generations != (Generations{7, 9}) || degraded.Snapshot.Physical("grid.c1.r1").Known {
		t.Fatal("disconnect retained state")
	}
	select {
	case ticks <- time.Time{}:
	case <-time.After(3 * time.Second):
		t.Fatal("retry blocked")
	}
	failed := awaitManaged(t, states, ManagedDegraded)
	if failed.Diagnostic.Code != device.ERR_DEVICE_PERMISSION || failed.Snapshot.Generations != (Generations{7, 9}) {
		t.Fatal(failed)
	}
	select {
	case ticks <- time.Time{}:
	case <-time.After(3 * time.Second):
		t.Fatal("retry blocked")
	}
	next := awaitManaged(t, states, ManagedConnected)
	if next.Snapshot.Generations != (Generations{8, 9}) || next.Snapshot.Availability != Unconfirmed || len(next.Snapshot.Controls()) != 0 {
		t.Fatal("reopen baseline fabricated")
	}
	if _, err := w2.Write(physicalBytes(8, 0, 20)); err != nil {
		t.Fatal(err)
	}
	fresh := awaitSnapshot(t, events)
	if fresh.Generations != (Generations{8, 9}) || fresh.Physical("grid.c2.r1") != (PhysicalState{false, true}) || len(fresh.Controls()) != 1 || fresh.Physical("grid.c1.r1").Known || !old.Physical("grid.c1.r1").Known {
		t.Fatal("generation contamination")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	awaitDone(t, m.Done())
	stopped := awaitManaged(t, states, ManagedStopped)
	if stopped.Snapshot.Generations != (Generations{8, 9}) {
		t.Fatal(stopped)
	}
	for _, node := range append(first, second...) {
		if _, err := node.File.Stat(); !errors.Is(err, os.ErrClosed) {
			t.Fatal("fd leaked")
		}
	}
}
func TestPhysicalManagedPermissionAndAbsence(t *testing.T) {
	for _, tc := range []struct {
		name            string
		absent, recover bool
	}{{"permission", false, true}, {"absent", true, true}, {"permission-cancel", false, false}, {"absent-cancel", true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			r := managedDiscovery("/synthetic/hidraw1")
			target, err := device.NewReconnectTarget(r, r.Groups[0])
			if err != nil {
				t.Fatal(err)
			}
			states := make(chan ManagedSnapshot, 4)
			events := make(chan *Snapshot, 1)
			ticks := make(chan time.Time)
			nodes, writer := pipeNode(t)
			recovered := false
			opens := 0
			m, err := startManaged(context.Background(), target, Generations{7, 9}, managedOps{discover: func() (*device.Result, *device.Diagnostic) {
				if tc.absent && !recovered {
					return &device.Result{}, nil
				}
				return r, nil
			}, session: sessionOps{open: func(device.Group) ([]device.OpenedNode, error) {
				opens++
				if !recovered {
					return nil, syscall.EACCES
				}
				return nodes, nil
			}, observe: func(s *Snapshot) { events <- s }}, retry: ticks, observe: func(s ManagedSnapshot) { states <- s }})
			if err != nil {
				t.Fatal(err)
			}
			defer m.Close()
			state := awaitManaged(t, states, ManagedDegraded)
			code := device.ERR_DEVICE_PERMISSION
			if tc.absent {
				code = device.ERR_DEVICE_NOT_FOUND
			}
			if state.Diagnostic.Code != code || state.Snapshot.Generations != (Generations{7, 9}) {
				t.Fatal(state)
			}
			if tc.absent && opens != 0 {
				t.Fatal("unselected device opened")
			}
			if tc.recover {
				recovered = true
				select {
				case ticks <- time.Time{}:
				case <-time.After(3 * time.Second):
					t.Fatal("retry blocked")
				}
				connected := awaitManaged(t, states, ManagedConnected)
				if connected.Snapshot.Generations != (Generations{7, 9}) || connected.Snapshot.Availability != Unconfirmed || len(connected.Snapshot.Controls()) != 0 {
					t.Fatal("recovery baseline fabricated", connected)
				}
				if _, err := writer.Write(physicalBytes(4, 1, 1)); err != nil {
					t.Fatal(err)
				}
				report := awaitSnapshot(t, events)
				if report.Generations != (Generations{7, 9}) || report.Physical("grid.c1.r1") != (PhysicalState{true, true}) || len(report.Controls()) != 1 {
					t.Fatal("recovery report", report)
				}
			}
			if err := m.Close(); err != nil {
				t.Fatal(err)
			}
			awaitDone(t, m.Done())
			stopped := awaitManaged(t, states, ManagedStopped)
			if stopped.Snapshot.Generations != (Generations{7, 9}) {
				t.Fatal(stopped)
			}
			if tc.recover {
				if _, err := nodes[0].File.Stat(); !errors.Is(err, os.ErrClosed) {
					t.Fatal("recovered fd leaked")
				}
			}
		})
	}
}
func TestPhysicalSessionCloseFailure(t *testing.T) {
	nodes, _ := pipeNode(t)
	events := make(chan *Snapshot, 1)
	s, err := startSession(context.Background(), device.Group{}, Generations{}, sessionOps{open: func(device.Group) ([]device.OpenedNode, error) { return nodes, nil }, observe: func(s *Snapshot) { events <- s }})
	if err != nil {
		t.Fatal(err)
	}
	if err := nodes[0].File.Close(); err != nil {
		t.Fatal(err)
	}
	awaitDone(t, s.Done())
	if s.Close() == nil {
		t.Fatal("close error lost")
	}
}
