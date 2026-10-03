package input

import (
	"context"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/device"
)

func TestManagedChangesAndMetrics(t *testing.T) {
	r := managedDiscovery("/synthetic/hidraw1")
	target, err := device.NewReconnectTarget(r, r.Groups[0])
	if err != nil {
		t.Fatal(err)
	}
	nodes, writer := pipeNode(t)
	events := make(chan *Snapshot, 8)
	states := make(chan ManagedSnapshot, 4)
	ticks := make(chan time.Time)
	m, err := startManaged(context.Background(), target, Generations{Device: 1}, managedOps{
		discover: func() (*device.Result, *device.Diagnostic) { return r, nil },
		session:  sessionOps{open: func(device.Group) ([]device.OpenedNode, error) { return nodes, nil }, observe: func(s *Snapshot) { events <- s }},
		retry:    ticks, observe: func(s ManagedSnapshot) { states <- s },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	awaitManaged(t, states, ManagedConnected)
	for _, report := range [][]byte{physicalBytes(4, 1, 1), physicalBytes(8, 1, 2), physicalBytes(12, 1, 5), physicalBytes(4, 3, 6)} {
		if _, err := writer.Write(report); err != nil {
			t.Fatal(err)
		}
		awaitSnapshot(t, events)
	}
	latest := m.Latest().Snapshot
	if latest.EventCount != 3 || latest.InvalidationCount != 2 || !latest.ReadAt.IsZero() || len(latest.Controls()) != 0 {
		t.Fatalf("invalid metrics: %+v", latest)
	}
	if len(m.changes) != 1 {
		t.Fatal("changes not coalesced")
	}
	<-m.Changes()
	if _, err := writer.Write(physicalBytes(4, 0, 7)); err != nil {
		t.Fatal(err)
	}
	awaitSnapshot(t, events)
	latest = m.Latest().Snapshot
	if latest.EventCount != 4 || latest.InvalidationCount != 2 || latest.ReadAt.IsZero() || latest.Physical("grid.c1.r1") != (PhysicalState{Known: true}) {
		t.Fatalf("accepted report: %+v", latest)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	awaitManaged(t, states, ManagedDegraded)
	latest = m.Latest().Snapshot
	if latest.Connected || !latest.ReadAt.IsZero() || latest.EventCount != 4 || latest.InvalidationCount != 2 {
		t.Fatalf("disconnect: %+v", latest)
	}
}
