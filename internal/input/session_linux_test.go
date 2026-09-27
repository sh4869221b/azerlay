package input

import (
	"context"
	"errors"
	"github.com/sh4869221b/azerlay/internal/device"
	"os"
	"sync"
	"testing"
	"time"
)

func pipeNode(t *testing.T) ([]device.OpenedNode, *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })
	return []device.OpenedNode{{File: r}}, w
}
func awaitSnapshot(t *testing.T, c <-chan *Snapshot) *Snapshot {
	t.Helper()
	select {
	case s := <-c:
		return s
	case <-time.After(3 * time.Second):
		t.Fatal("publication blocked")
		return nil
	}
}
func awaitDone(t *testing.T, c <-chan struct{}) {
	t.Helper()
	select {
	case <-c:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown blocked")
	}
}
func TestPhysicalSessionObservationLifecycle(t *testing.T) {
	nodes, w := pipeNode(t)
	observed := make(chan *Snapshot, 16)
	s, err := startSession(context.Background(), device.Group{}, Generations{7, 9}, sessionOps{open: func(device.Group) ([]device.OpenedNode, error) { return nodes, nil }, observe: func(s *Snapshot) { observed <- s }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if initial := s.Latest(); initial.Availability != Unconfirmed || initial.Physical("grid.c1.r1").Known {
		t.Fatal("initial knowledge fabricated")
	}
	send := func(b []byte) *Snapshot {
		t.Helper()
		if _, err := w.Write(b); err != nil {
			t.Fatal(err)
		}
		return awaitSnapshot(t, observed)
	}
	held := send(physicalBytes(4, 1, 10))
	if held.Physical("grid.c1.r1") != (PhysicalState{true, true}) {
		t.Fatal(held)
	}
	held.Controls()["grid.c1.r1"] = PhysicalState{}
	held.Sequence = 999
	second := send(physicalBytes(8, 0, 11))
	if !second.Physical("grid.c1.r1").Down || second.Sequence != 2 {
		t.Fatal("snapshot mutated")
	}
	gap := send(physicalBytes(12, 1, 14))
	if gap.Physical("grid.c1.r1").Known || !gap.Physical("grid.c3.r1").Known || gap.Reason != ERR_INPUT_DROPPED {
		t.Fatal("loss retained old state")
	}
	malformed := physicalBytes(4, 3, 15)
	invalid := send(malformed)
	if len(invalid.Controls()) != 0 || invalid.Availability != Unconfirmed {
		t.Fatal("invalid report retained state")
	}
	released := send(physicalBytes(4, 0, 16))
	if released.Physical("grid.c1.r1") != (PhysicalState{false, true}) || released.Physical("grid.c3.r1").Known {
		t.Fatal("recovery fabricated controls")
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	awaitDone(t, s.Done())
	if final := s.Latest(); final.Connected || len(final.Controls()) != 0 || final.Availability != Unavailable || final.Generations != (Generations{7, 9}) {
		t.Fatal(final)
	}
	if s.Err() == nil || s.Err().Error() != ERR_INPUT_READ {
		t.Fatal(s.Err())
	}
	if _, err := nodes[0].File.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("reader leaked")
	}
	if !held.Physical("grid.c1.r1").Down {
		t.Fatal("retained snapshot changed")
	}
}
func TestPhysicalSessionIdleClose(t *testing.T) {
	nodes, _ := pipeNode(t)
	ctx, cancel := context.WithCancel(context.Background())
	s, err := startSession(ctx, device.Group{}, Generations{}, sessionOps{open: func(device.Group) ([]device.OpenedNode, error) { return nodes, nil }})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	awaitDone(t, s.Done())
	var group sync.WaitGroup
	for range 4 {
		group.Go(func() {
			if err := s.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
	if _, err := nodes[0].File.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("idle fd leaked")
	}
}
func TestPhysicalSessionCancellationDuringOpen(t *testing.T) {
	nodes, _ := pipeNode(t)
	ctx, cancel := context.WithCancel(context.Background())
	s, err := startSession(ctx, device.Group{}, Generations{}, sessionOps{open: func(device.Group) ([]device.OpenedNode, error) { cancel(); return nodes, nil }})
	if s != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("%v %v", s, err)
	}
	if _, err := nodes[0].File.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("rollback fd leaked")
	}
}
func TestPhysicalCounterResetAndSilence(t *testing.T) {
	for _, counter := range []byte{0, 9, 10} {
		r := newReducer(Generations{})
		r.apply(PhysicalEvent{RegionID: "one", Down: true, counter: 10})
		s := r.apply(PhysicalEvent{RegionID: "two", counter: counter})
		if s.Physical("one").Known || !s.Physical("two").Known {
			t.Fatal("reset retained prior state")
		}
	}
	r := newReducer(Generations{})
	r.apply(PhysicalEvent{RegionID: "one", Down: true, counter: 255})
	wrapped := r.apply(PhysicalEvent{RegionID: "two", counter: 0})
	if wrapped.Physical("one").Known {
		t.Fatal("wrap assumed qualified")
	}
	// No time-based release exists: unrelated traffic cannot clear a held control.
	nodes, w := pipeNode(t)
	events := make(chan *Snapshot, 4)
	s, err := startSession(context.Background(), device.Group{}, Generations{}, sessionOps{open: func(device.Group) ([]device.OpenedNode, error) { return nodes, nil }, observe: func(s *Snapshot) { events <- s }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := w.Write(physicalBytes(4, 1, 1)); err != nil {
		t.Fatal(err)
	}
	awaitSnapshot(t, events)
	unrelated := physicalBytes(8, 0, 2)
	unrelated[2] = 18
	if _, err := w.Write(unrelated); err != nil {
		t.Fatal(err)
	}
	if !s.Latest().Physical("grid.c1.r1").Down {
		t.Fatal("silence fabricated release")
	}
}
