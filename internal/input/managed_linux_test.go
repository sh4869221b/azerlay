package input

import (
	"context"
	"os"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/device"
)

func managedDiscovery(paths ...string) *device.Result {
	parent, serial := "/synthetic/usb/unit", "synthetic-unit"
	r := &device.Result{Groups: []device.Group{{USBParent: parent, EventPaths: paths}}}
	for i, path := range paths {
		node := device.Node{Path: path, USBParent: &parent, Serial: &serial, USBID: &device.USBID{Vendor: 0x16d0, Product: 0x12f7, Release: 0x111}, InputID: &device.InputID{Bus: 3, Vendor: 0x16d0, Product: 0x12f7, Version: 0x111}, Admission: "admitted", Access: "readable"}
		if i == 0 {
			node.Interface = &device.Interface{Number: 1, Class: 3}
			node.Roles = []string{"keyboard", "relative", "absolute"}
			node.Capabilities = map[string][]int{"ev": {0, 1, 2, 3}, "key": {30}, "rel": {6}, "abs": {0, 1}}
		} else {
			node.Interface = &device.Interface{Number: 2, Class: 3, Subclass: 1, Protocol: 2}
			node.Roles = []string{"mouse_buttons", "relative"}
			node.Capabilities = map[string][]int{"ev": {0, 1, 2}, "key": {272}, "rel": {0, 1}}
		}
		r.Nodes = append(r.Nodes, node)
	}
	return r
}

func managedPipeOps(nodes []device.OpenedNode, events chan integrationObservation) sessionOps {
	return sessionOps{
		open:  func(device.Group) ([]device.OpenedNode, error) { return nodes, nil },
		clock: func(*os.File) error { return nil },
		axes: func(*os.File, []int) (map[uint16]AxisInfo, error) {
			return map[uint16]AxisInfo{0: {Minimum: -100, Maximum: 100}, 1: {Minimum: -100, Maximum: 100}}, nil
		},
		observe: func(node int, event Event, snapshot *Snapshot) {
			events <- integrationObservation{node, event, snapshot}
		},
	}
}

func awaitManaged(t *testing.T, states <-chan ManagedSnapshot, want ManagedState) ManagedSnapshot {
	t.Helper()
	select {
	case state := <-states:
		if state.State != want || state.Snapshot == nil || state.Snapshot.Connected != (want == ManagedConnected) {
			t.Fatalf("managed state=%+v, want %s", state, want)
		}
		return state
	case <-time.After(3 * time.Second):
		t.Fatal("managed transition blocked")
		return ManagedSnapshot{}
	}
}

func tickManaged(t *testing.T, ticks chan<- time.Time) {
	t.Helper()
	select {
	case ticks <- time.Time{}:
	case <-time.After(3 * time.Second):
		t.Fatal("managed retry blocked")
	}
}

func activeManagedSession(m *Managed) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.session
}

func TestManagedPermissionReturn(t *testing.T) {
	t.Parallel()
	r := managedDiscovery("/synthetic/event1")
	target, err := device.NewReconnectTarget(r, r.Groups[0])
	if err != nil {
		t.Fatal(err)
	}
	observed := make(chan ManagedSnapshot, 4)
	ticks := make(chan time.Time)
	nodes, writers := pipeNodes(t, 1)
	nodes[0].Node = r.Nodes[0]
	events := make(chan integrationObservation, 8)
	ops := managedPipeOps(nodes, events)
	attempts := 0
	ops.open = func(device.Group) ([]device.OpenedNode, error) {
		attempts++
		if attempts == 1 {
			return nil, syscall.EACCES
		}
		return nodes, nil
	}
	m, err := startManaged(context.Background(), target, Generations{Device: 7, Profile: 9}, managedOps{
		discover: func() (*device.Result, *device.Diagnostic) { return r, nil },
		session:  ops,
		retry:    ticks, observe: func(s ManagedSnapshot) { observed <- s },
	})
	if err != nil || m == nil {
		t.Fatalf("valid denied target should remain live: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	state := awaitManaged(t, observed, ManagedDegraded)
	if state.Diagnostic == nil || state.Diagnostic.Code != device.ERR_DEVICE_PERMISSION || state.Diagnostic.Remediation == "" {
		t.Fatalf("permission state=%+v", state)
	}
	state.Diagnostic.Remediation = "mutated"
	if m.Latest().Diagnostic.Remediation == "mutated" {
		t.Fatal("diagnostic storage exposed")
	}
	tickManaged(t, ticks)
	connected := awaitManaged(t, observed, ManagedConnected)
	if connected.Snapshot.Generations != (Generations{Device: 7, Profile: 9}) || connected.Snapshot.Sequence != 0 {
		t.Fatalf("first success generation=%+v", connected.Snapshot)
	}
	if _, known := connected.Snapshot.Key(0, 30); known {
		t.Fatal("startup fabricated input state")
	}
	s := integrationSession{activeManagedSession(m), nodes, writers, events}
	s.send(t, 0, Event{Type: EV_KEY, Code: 30, Value: 1}, Event{Type: EV_SYN})
	if down, known := m.Latest().Snapshot.Key(0, 30); !down || !known {
		t.Fatal("permission return did not resume input")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	awaitManaged(t, observed, ManagedStopped)
	assertFilesClosed(t, nodes)
	if attempts != 2 {
		t.Fatalf("open attempts=%d", attempts)
	}
}

func TestManagedReconnect(t *testing.T) {
	t.Parallel()
	r := managedDiscovery("/synthetic/event1", "/synthetic/event2")
	selected := r.Groups[0]
	selected.EventPaths = []string{r.Nodes[1].Path, r.Nodes[0].Path}
	target, err := device.NewReconnectTarget(r, selected)
	if err != nil {
		t.Fatal(err)
	}
	oldNodes, oldWriters := pipeNodes(t, 2)
	newNodes, newWriters := pipeNodes(t, 2)
	for i := range oldNodes {
		oldNodes[i].Node = r.Nodes[1-i]
	}
	events := make(chan integrationObservation, 16)
	states := make(chan ManagedSnapshot, 8)
	ticks := make(chan time.Time)
	ops := managedPipeOps(oldNodes, events)
	var old *Session
	attempts := 0
	ops.open = func(g device.Group) ([]device.OpenedNode, error) {
		attempts++
		if !slices.Equal(g.EventPaths, []string{r.Nodes[1].Path, r.Nodes[0].Path}) {
			return nil, &InputError{Code: ERR_INPUT_EVENT}
		}
		if attempts == 1 {
			return oldNodes, nil
		}
		select {
		case <-old.Done():
		default:
			return nil, &InputError{Code: ERR_INPUT_EVENT}
		}
		if attempts == 2 {
			return nil, syscall.EACCES
		}
		return newNodes, nil
	}
	m, err := startManaged(context.Background(), target, Generations{Device: 3, Profile: 5}, managedOps{discover: func() (*device.Result, *device.Diagnostic) { return r, nil }, session: ops, retry: ticks, observe: func(s ManagedSnapshot) { states <- s }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	awaitManaged(t, states, ManagedConnected)
	old = activeManagedSession(m)
	s := integrationSession{old, oldNodes, oldWriters, events}
	retained := s.send(t, 0, Event{Type: EV_KEY, Code: 30, Value: 1}, Event{Type: EV_SYN})
	s.send(t, 1, Event{Type: EV_ABS, Value: 42})
	if err := oldWriters[0].Close(); err != nil {
		t.Fatal(err)
	}
	degraded := awaitManaged(t, states, ManagedDegraded)
	if degraded.Diagnostic.Code != ERR_INPUT_READ || degraded.Snapshot.Sequence != 2 {
		t.Fatalf("disconnect = %+v", degraded)
	}
	if _, known := degraded.Snapshot.Key(0, 30); known {
		t.Fatal("disconnect retained pressed state")
	}
	assertFilesClosed(t, oldNodes)
	r = managedDiscovery("/synthetic/event11", "/synthetic/event22")
	for i := range newNodes {
		newNodes[i].Node = r.Nodes[1-i]
	}
	tickManaged(t, ticks)
	denied := awaitManaged(t, states, ManagedDegraded)
	if denied.Diagnostic.Code != device.ERR_DEVICE_PERMISSION || denied.Snapshot.Generations.Device != 3 {
		t.Fatalf("failed retry advanced generation: %+v", denied)
	}
	tickManaged(t, ticks)
	connected := awaitManaged(t, states, ManagedConnected)
	if connected.Snapshot.Generations != (Generations{Device: 4, Profile: 5}) || connected.Snapshot.Sequence != 0 {
		t.Fatalf("replacement = %+v", connected)
	}
	next := integrationSession{activeManagedSession(m), newNodes, newWriters, events}
	if next.Session == old {
		t.Fatal("old session reused")
	}
	if _, known := connected.Snapshot.Key(0, 30); known {
		t.Fatal("replacement inherited old key")
	}
	next.send(t, 0, Event{Type: EV_KEY, Code: 30, Value: 0}, Event{Type: EV_SYN})
	latest := m.Latest()
	if down, known := latest.Snapshot.Key(0, 30); down || !known || latest.Snapshot.Generations.Device != 4 {
		t.Fatal("replacement did not resume in selected slot")
	}
	if down, known := retained.Key(0, 30); !down || !known || old.Latest().Sequence != 2 {
		t.Fatal("replacement mutated old session")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	awaitManaged(t, states, ManagedStopped)
	assertFilesClosed(t, newNodes)
	if attempts != 3 {
		t.Fatalf("attempts=%d", attempts)
	}
	select {
	case ticks <- time.Time{}:
		t.Fatal("retry accepted after Close")
	default:
	}
}
