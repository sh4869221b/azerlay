package input

import (
	"context"
	"github.com/sh4869221b/azerlay/internal/device"
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
		return second, nil
	}, observe: func(s *Snapshot) { events <- s }}, retry: ticks, observe: func(s ManagedSnapshot) { states <- s }})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	awaitManaged(t, states, ManagedConnected)
	if _, err := w1.Write(physicalBytes(4, 1, 1)); err != nil {
		t.Fatal(err)
	}
	old := awaitSnapshot(t, events)
	if err := w1.Close(); err != nil {
		t.Fatal(err)
	}
	degraded := awaitManaged(t, states, ManagedDegraded)
	if degraded.Snapshot.Physical("grid.c1.r1").Known {
		t.Fatal("disconnect retained state")
	}
	select {
	case ticks <- time.Time{}:
	case <-time.After(3 * time.Second):
		t.Fatal("retry blocked")
	}
	next := awaitManaged(t, states, ManagedConnected)
	if next.Snapshot.Generations.Device != 8 || next.Snapshot.Availability != Unconfirmed || len(next.Snapshot.Controls()) != 0 {
		t.Fatal("reopen baseline fabricated")
	}
	if _, err := w2.Write(physicalBytes(8, 0, 20)); err != nil {
		t.Fatal(err)
	}
	fresh := awaitSnapshot(t, events)
	if fresh.Generations.Device != 8 || fresh.Physical("grid.c1.r1").Known || !old.Physical("grid.c1.r1").Known {
		t.Fatal("generation contamination")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	awaitManaged(t, states, ManagedStopped)
	for _, node := range append(first, second...) {
		if _, err := node.File.Stat(); err == nil {
			t.Fatal("fd leaked")
		}
	}
}
func TestPhysicalManagedPermissionAndAbsence(t *testing.T) {
	for _, absent := range []bool{false, true} {
		t.Run(map[bool]string{false: "permission", true: "absent"}[absent], func(t *testing.T) {
			r := managedDiscovery("/synthetic/hidraw1")
			target, err := device.NewReconnectTarget(r, r.Groups[0])
			if err != nil {
				t.Fatal(err)
			}
			states := make(chan ManagedSnapshot, 4)
			opens := 0
			m, err := startManaged(context.Background(), target, Generations{}, managedOps{discover: func() (*device.Result, *device.Diagnostic) {
				if absent {
					return &device.Result{}, nil
				}
				return r, nil
			}, session: sessionOps{open: func(device.Group) ([]device.OpenedNode, error) { opens++; return nil, syscall.EACCES }}, retry: make(chan time.Time), observe: func(s ManagedSnapshot) { states <- s }})
			if err != nil {
				t.Fatal(err)
			}
			state := awaitManaged(t, states, ManagedDegraded)
			code := device.ERR_DEVICE_PERMISSION
			if absent {
				code = device.ERR_DEVICE_NOT_FOUND
			}
			if state.Diagnostic.Code != code {
				t.Fatal(state)
			}
			if err := m.Close(); err != nil {
				t.Fatal(err)
			}
			if absent && opens != 0 {
				t.Fatal("unselected device opened")
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
