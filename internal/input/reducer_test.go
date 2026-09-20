package input

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func applyPending(t *testing.T, r *reducer, node int, events ...Event) {
	t.Helper()
	for _, event := range events {
		snapshot, err := r.apply(node, event)
		if err != nil || snapshot != nil {
			t.Fatalf("pending event published or failed: snapshot=%v err=%v", snapshot, err)
		}
	}
}

func reportFrame(t *testing.T, r *reducer, node int, timestamp time.Duration) *Snapshot {
	t.Helper()
	snapshot, err := r.apply(node, Event{Timestamp: timestamp, Type: EV_SYN, Code: SYN_REPORT})
	if err != nil || snapshot == nil {
		t.Fatalf("report did not publish: snapshot=%v err=%v", snapshot, err)
	}
	return snapshot
}

func TestReducerFrameSemantics(t *testing.T) {
	t.Parallel()
	generations := Generations{Device: 7, Profile: 9}
	r := newReducer(1, generations)
	frame := []Event{
		{Timestamp: 1, Type: EV_MSC, Code: MSC_SCAN, Value: 123},
		{Timestamp: 2, Type: EV_KEY, Code: 30, Value: 1},
		{Timestamp: 3, Type: EV_KEY, Code: 30, Value: 2},
		{Timestamp: 4, Type: EV_KEY, Code: 30, Value: 0},
		{Timestamp: 5, Type: EV_ABS, Code: 0, Value: -32768},
		{Timestamp: 6, Type: EV_REL, Code: 0, Value: 2147483647},
		{Timestamp: 7, Type: EV_REL, Code: 0, Value: 2},
		{Timestamp: 8, Type: EV_REL, Code: 1, Value: -4},
		{Timestamp: 9, Type: EV_REL, Code: 8, Value: 1},
		{Timestamp: 10, Type: EV_REL, Code: 11, Value: 120},
	}
	applyPending(t, r, 0, frame...)
	snapshot := reportFrame(t, r, 0, 11)
	if snapshot.Sequence != 1 || snapshot.Timestamp != 11 || !snapshot.Connected || snapshot.Generations != generations {
		t.Fatalf("unexpected publication metadata: %+v", snapshot)
	}
	if node, ok := snapshot.ReportingNode(); !ok || node != 0 {
		t.Fatalf("reporting node = %d, %t", node, ok)
	}
	if down, known := snapshot.Key(0, 30); down || !known {
		t.Fatalf("released key = %t, %t", down, known)
	}
	if value, known := snapshot.Absolute(0, 0); value != -32768 || !known {
		t.Fatalf("raw axis = %d, %t", value, known)
	}
	for code, want := range map[uint16]int64{0: 2147483649, 1: -4, 8: 1, 11: 120} {
		if got, ok := snapshot.Relative(code); !ok || got != want {
			t.Errorf("relative code %d = %d, %t; want %d", code, got, ok, want)
		}
	}
	wantTransitions := []KeyTransition{
		{Timestamp: 2, Code: 30, Action: KeyPress},
		{Timestamp: 3, Code: 30, Action: KeyRepeat},
		{Timestamp: 4, Code: 30, Action: KeyRelease},
	}
	if got := snapshot.KeyTransitions(); !reflect.DeepEqual(got, wantTransitions) {
		t.Errorf("transitions = %v; want %v", got, wantTransitions)
	}
	if got := snapshot.FrameEvents(); !reflect.DeepEqual(got, frame) {
		t.Errorf("frame = %v; want %v", got, frame)
	}
	empty := reportFrame(t, r, 0, 12)
	if empty.Sequence != 2 || len(empty.FrameEvents()) != 0 || len(empty.KeyTransitions()) != 0 {
		t.Fatalf("empty report retained a frame: %+v", empty)
	}
	if _, ok := empty.Relative(0); ok {
		t.Error("relative delta persisted into next frame")
	}
	if down, known := empty.Key(0, 30); down || !known {
		t.Error("empty report lost observed released state")
	}
	if value, known := empty.Absolute(0, 0); !known || value != -32768 {
		t.Error("empty report lost absolute state")
	}
}

func TestReducerNodeIsolationAndMonotonicTime(t *testing.T) {
	t.Parallel()
	r := newReducer(2, Generations{Device: 3, Profile: 4})
	applyPending(t, r, 0, Event{Timestamp: 20, Type: EV_KEY, Code: 30, Value: 1}, Event{Timestamp: 21, Type: EV_ABS, Code: 0, Value: -1})
	applyPending(t, r, 1, Event{Timestamp: 10, Type: EV_KEY, Code: 30, Value: 0}, Event{Timestamp: 11, Type: EV_ABS, Code: 0, Value: 2})
	b := reportFrame(t, r, 1, 12)
	if _, known := b.Key(0, 30); known {
		t.Fatal("node A's pending key leaked into node B's report")
	}
	if _, known := b.Absolute(0, 0); known {
		t.Fatal("node A's pending axis leaked into node B's report")
	}
	if b.Sequence != 1 || b.Timestamp != 12 {
		t.Fatalf("B metadata = %+v", b)
	}
	a := reportFrame(t, r, 0, 22)
	if down, known := a.Key(0, 30); !down || !known {
		t.Fatal("node A's key was not committed")
	}
	if down, known := a.Key(1, 30); down || !known {
		t.Fatal("same-code node B key was overwritten by A")
	}
	for node, want := range []int32{-1, 2} {
		if got, known := a.Absolute(node, 0); !known || got != want {
			t.Fatalf("node %d absolute = %d, %t; want %d", node, got, known, want)
		}
	}
	applyPending(t, r, 1, Event{Timestamp: 13, Type: EV_KEY, Code: 30, Value: 2})
	repeat := reportFrame(t, r, 1, 14)
	if repeat.Sequence != 3 || repeat.Timestamp != 22 {
		t.Fatalf("publication time regressed: %+v", repeat)
	}
	if down, known := repeat.Key(1, 30); !down || !known {
		t.Fatal("repeat did not establish down state")
	}
	if got := repeat.KeyTransitions(); !reflect.DeepEqual(got, []KeyTransition{{Timestamp: 13, Code: 30, Action: KeyRepeat}}) {
		t.Fatalf("repeat transition lost original timestamp: %v", got)
	}
}

func TestReducerIgnoresIrrelevantEvents(t *testing.T) {
	t.Parallel()
	r := newReducer(1, Generations{})
	applyPending(t, r, 0,
		Event{Type: EV_SYN, Code: 2}, Event{Type: 5, Code: 30, Value: 1},
		Event{Type: EV_MSC, Code: 0, Value: 30}, Event{Type: EV_KEY, Code: 0xffff, Value: 1},
		Event{Type: EV_ABS, Code: 0xffff, Value: 1}, Event{Type: EV_REL, Code: 0x0a, Value: 1},
		Event{Type: EV_REL, Code: 0xffff, Value: 1},
	)
	snapshot := reportFrame(t, r, 0, 1)
	if len(snapshot.FrameEvents()) != 0 {
		t.Fatal("irrelevant events were retained")
	}
	if _, known := snapshot.Key(0, 30); known {
		t.Fatal("irrelevant event established key state")
	}
}

func TestReducerRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		event Event
		code  string
	}{
		{"negative key", Event{Type: EV_KEY, Code: 30, Value: -1}, ERR_INPUT_EVENT},
		{"unknown key value", Event{Type: EV_KEY, Code: 30, Value: 3}, ERR_INPUT_EVENT},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newReducer(1, Generations{})
			applyPending(t, r, 0, Event{Type: EV_KEY, Code: 30, Value: 1})
			snapshot, err := r.apply(0, tc.event)
			var inputErr *InputError
			if snapshot != nil || !errors.As(err, &inputErr) || inputErr.Code != tc.code || err.Error() != tc.code {
				t.Fatalf("invalid input: snapshot=%v err=%v; want %s", snapshot, err, tc.code)
			}
			if _, known := r.stop().Key(0, 30); known {
				t.Fatal("failed frame's key became known")
			}
		})
	}
}

func TestReducerStopClearsAndPublishesOnce(t *testing.T) {
	t.Parallel()
	generations := Generations{Device: 8, Profile: 13}
	r := newReducer(2, generations)
	applyPending(t, r, 0, Event{Type: EV_KEY, Code: 30, Value: 1}, Event{Type: EV_ABS, Code: 0, Value: 42}, Event{Type: EV_REL, Code: 0, Value: 2})
	reportFrame(t, r, 0, 50)
	applyPending(t, r, 1, Event{Type: EV_KEY, Code: 31, Value: 1})
	terminal := r.stop()
	if terminal.Connected || terminal.Sequence != 2 || terminal.Timestamp != 50 || terminal.Generations != generations {
		t.Fatalf("terminal metadata = %+v", terminal)
	}
	for _, key := range []struct {
		node int
		code uint16
	}{{0, 30}, {1, 31}} {
		if down, known := terminal.Key(key.node, key.code); down || known {
			t.Fatal("terminal retained committed or pending key state")
		}
	}
	if _, known := terminal.Absolute(0, 0); known {
		t.Fatal("terminal retained raw axis")
	}
	if _, known := terminal.Relative(0); known {
		t.Fatal("terminal retained relative frame")
	}
	if _, ok := terminal.ReportingNode(); ok || len(terminal.FrameEvents()) != 0 || len(terminal.KeyTransitions()) != 0 {
		t.Fatal("terminal retained reporting frame")
	}
	if r.stop() != nil {
		t.Fatal("second stop published again")
	}
	applyPending(t, r, 0, Event{Type: EV_SYN, Code: SYN_REPORT})
}
