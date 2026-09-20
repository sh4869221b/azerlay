package input

import (
	"reflect"
	"testing"
)

func TestSnapshotInitialStateIsUnknown(t *testing.T) {
	t.Parallel()
	generations := Generations{Device: 1, Profile: 2}
	snapshot := newReducer(2, generations).snapshot()
	if snapshot.Sequence != 0 || snapshot.Timestamp != 0 || !snapshot.Connected || snapshot.Generations != generations {
		t.Fatalf("initial snapshot = %+v", snapshot)
	}
	for _, node := range []int{-1, 0, 1, 2} {
		if down, known := snapshot.Key(node, 30); down || known {
			t.Fatalf("unobserved key on node %d = %t, %t", node, down, known)
		}
		if value, known := snapshot.Absolute(node, 0); value != 0 || known {
			t.Fatalf("unobserved axis on node %d = %d, %t", node, value, known)
		}
	}
	if _, ok := snapshot.ReportingNode(); ok || len(snapshot.FrameEvents()) != 0 || len(snapshot.KeyTransitions()) != 0 {
		t.Fatal("initial snapshot has a reporting frame")
	}
}

func TestSnapshotRetainedStateAndAccessorIsolation(t *testing.T) {
	t.Parallel()
	r := newReducer(1, Generations{Device: 1, Profile: 2})
	frame := []Event{
		{Timestamp: 1, Type: EV_KEY, Code: 30, Value: 1},
		{Timestamp: 2, Type: EV_ABS, Code: 0, Value: 9},
		{Timestamp: 3, Type: EV_REL, Code: 0, Value: 4},
	}
	applyPending(t, r, 0, frame...)
	old := reportFrame(t, r, 0, 4)
	events := old.FrameEvents()
	events[0].Value = 0
	transitions := old.KeyTransitions()
	transitions[0].Action = KeyRelease
	applyPending(t, r, 0, Event{Type: EV_KEY, Code: 30, Value: 0}, Event{Type: EV_ABS, Code: 0, Value: -9}, Event{Type: EV_REL, Code: 0, Value: -4})
	reportFrame(t, r, 0, 5)
	r.stop()
	if !old.Connected || old.Sequence != 1 || old.Timestamp != 4 {
		t.Fatalf("retained snapshot metadata changed: %+v", old)
	}
	if down, known := old.Key(0, 30); !down || !known {
		t.Fatal("later reduction changed retained key")
	}
	if value, known := old.Absolute(0, 0); !known || value != 9 {
		t.Fatal("later reduction changed retained axis")
	}
	if value, known := old.Relative(0); !known || value != 4 {
		t.Fatal("later reduction changed retained relative data")
	}
	if got := old.FrameEvents(); !reflect.DeepEqual(got, frame) {
		t.Fatalf("frame storage was aliased: %v", got)
	}
	if got := old.KeyTransitions(); !reflect.DeepEqual(got, []KeyTransition{{Timestamp: 1, Code: 30, Action: KeyPress}}) {
		t.Fatalf("transition accessor storage was aliased: %v", got)
	}
}

func TestSnapshotScalarOwnership(t *testing.T) {
	t.Parallel()
	generations := Generations{Device: 1, Profile: 2}
	r := newReducer(1, generations)
	first := reportFrame(t, r, 0, 3)
	other := r.snapshot()
	first.Sequence = 99
	first.Timestamp = 100
	first.Connected = false
	first.Generations = Generations{Device: 99, Profile: 99}
	if other.Sequence != 1 || other.Timestamp != 3 || !other.Connected || other.Generations != generations {
		t.Fatalf("caller scalar mutation reached another reader: %+v", other)
	}
	next := reportFrame(t, r, 0, 4)
	if next.Sequence != 2 || next.Timestamp != 4 || !next.Connected || next.Generations != generations {
		t.Fatalf("caller scalar mutation reached reducer: %+v", next)
	}
}
