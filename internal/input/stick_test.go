package input

import (
	"math"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
)

func TestProjectStickXbox(t *testing.T) {
	t.Parallel()
	r := newReducer(2, Generations{Device: 3, Profile: 4})
	r.latest.axisInfo = []map[uint16]AxisInfo{{0: {Minimum: -100, Maximum: 100}, 1: {Minimum: -100, Maximum: 100}}, {0: {Minimum: -100, Maximum: 100}, 1: {Minimum: -100, Maximum: 100}}}
	binding := profile.StickBinding{Mode: profile.StickModeXbox}
	source := StickSource{Xbox: XboxStickSource{Node: 0, X: 0, Y: 1}}
	if state := ProjectStick(r.snapshot(), binding, source); state.Known || state.AxisX.Known || state.AxisX.Valid {
		t.Fatalf("initial = %+v", state)
	}
	applyPending(t, r, 0, Event{Type: EV_ABS, Code: 0, Value: 150}, Event{Type: EV_ABS, Code: 1, Value: -100})
	if state := ProjectStick(r.snapshot(), binding, source); state.Known {
		t.Fatal("pending frame became known")
	}
	old := reportFrame(t, r, 0, 1)
	state := ProjectStick(old, binding, source)
	if !state.Known || !state.AxisX.Valid || state.AxisX.Raw != 150 || state.X != 1 || state.Y != -1 || state.Intensity != 1 || math.Abs(state.DirectionX-1/math.Sqrt2) > 1e-12 || math.Abs(state.DirectionY+1/math.Sqrt2) > 1e-12 || state.Sequence != old.Sequence || state.Generations != old.Generations {
		t.Fatalf("diagonal = %+v", state)
	}
	if raw, known := old.Absolute(0, 0); raw != 150 || !known {
		t.Fatal("raw changed")
	}
	applyPending(t, r, 1, Event{Type: EV_ABS, Code: 0, Value: -100})
	reportFrame(t, r, 1, 2)
	other := source
	other.Xbox.Node = 1
	if state := ProjectStick(r.snapshot(), binding, other); state.Known || !state.AxisX.Known || state.AxisY.Known {
		t.Fatalf("nodes combined: %+v", state)
	}
	applyPending(t, r, 0, Event{Type: EV_ABS, Code: 0, Value: 30}, Event{Type: EV_ABS, Code: 1, Value: 40})
	next := reportFrame(t, r, 0, 3)
	if state := ProjectStick(next, binding, source); state.Intensity != .5 || state.DirectionX != .6 || state.DirectionY != .8 {
		t.Fatalf("partial = %+v", state)
	}
	if got := ProjectStick(old, binding, source); got != state {
		t.Fatal("retained snapshot changed")
	}
	for _, bad := range []XboxStickSource{{Node: -1, X: 0, Y: 1}, {Node: 2, X: 0, Y: 1}, {Node: 0, X: 0, Y: 0}, {Node: 0, X: 0, Y: 2}} {
		if got := ProjectStick(next, binding, StickSource{Xbox: bad}); got.Known {
			t.Fatalf("invalid selector = %+v", got)
		}
	}
	if got := ProjectStick(r.stop(), binding, source); got.Connected || got.Known || got.AxisX.Known {
		t.Fatalf("terminal = %+v", got)
	}
}

func TestProjectStickNonzeroAngle(t *testing.T) {
	t.Parallel()
	r := newReducer(1, Generations{Device: 3, Profile: 7})
	r.latest.axisInfo = []map[uint16]AxisInfo{{0: {Minimum: -100, Maximum: 100}, 1: {Minimum: -100, Maximum: 100}}}
	applyPending(t, r, 0, Event{Type: EV_ABS, Code: 0, Value: 50}, Event{Type: EV_ABS, Code: 1, Value: -100})
	snapshot := reportFrame(t, r, 0, 1)
	source := StickSource{Xbox: XboxStickSource{Node: 0, X: 0, Y: 1}}
	if got := ProjectStick(snapshot, profile.StickBinding{Mode: profile.StickModeXbox}, source); !got.Known || got.X != .5 || got.Y != -1 {
		t.Fatalf("zero angle = %+v, want projected axes", got)
	}
	want := StickState{Mode: profile.StickModeXbox, Connected: snapshot.Connected, Sequence: snapshot.Sequence, Generations: snapshot.Generations}
	if got := ProjectStick(snapshot, profile.StickBinding{Mode: profile.StickModeXbox, AngleDegrees: 90}, source); got != want {
		t.Fatalf("nonzero angle = %+v, want metadata and unknown zero vector %+v", got, want)
	}
}

func TestProjectStickAxisAvailability(t *testing.T) {
	t.Parallel()
	r := newReducer(1, Generations{})
	r.latest.axisInfo = []map[uint16]AxisInfo{{0: {}, 1: {Minimum: -1, Maximum: 1}}}
	applyPending(t, r, 0, Event{Type: EV_ABS, Code: 0, Value: 7}, Event{Type: EV_ABS, Code: 2, Value: 8})
	s := reportFrame(t, r, 0, 1)
	for _, tc := range []struct {
		code  uint16
		raw   int32
		known bool
	}{{0, 7, true}, {1, 0, false}, {2, 8, true}, {3, 0, false}} {
		raw, normalized, known, valid := s.NormalizedAbsolute(0, tc.code)
		if raw != tc.raw || normalized != 0 || known != tc.known || valid {
			t.Fatalf("axis %d = %d,%v,%t,%t", tc.code, raw, normalized, known, valid)
		}
	}
	state := ProjectStick(s, profile.StickBinding{Mode: profile.StickModeXbox}, StickSource{Xbox: XboxStickSource{X: 0, Y: 1}})
	if state.Known || !state.AxisX.Known || state.AxisX.Valid || state.AxisY.Known {
		t.Fatalf("invalid vs unknown = %+v", state)
	}
}

func TestProjectStickKeyboard(t *testing.T) {
	t.Parallel()
	r := newReducer(2, Generations{})
	binding := profile.StickBinding{Mode: profile.StickModeKeyboard, KeyboardDirections: profile.KeyboardDirections{Up: profile.KEY_W, Right: profile.KEY_D, Down: profile.KEY_S, Left: profile.KEY_A}}
	source := StickSource{Keyboard: KeyboardStickSource{Right: 1}}
	applyPending(t, r, 0, Event{Type: EV_KEY, Code: 17, Value: 1})
	partial := reportFrame(t, r, 0, 1)
	got := ProjectStick(partial, binding, source)
	if got.Known || !got.Up.Known || !got.Up.Down || got.Right.Known || got.Intensity != 0 {
		t.Fatalf("partial = %+v", got)
	}
	applyPending(t, r, 0, Event{Type: EV_KEY, Code: 31, Value: 0}, Event{Type: EV_KEY, Code: 30, Value: 0}, Event{Type: EV_KEY, Code: 32, Value: 0})
	reportFrame(t, r, 0, 2)
	applyPending(t, r, 1, Event{Type: EV_KEY, Code: 32, Value: 1})
	diagonal := reportFrame(t, r, 1, 3)
	got = ProjectStick(diagonal, binding, source)
	if !got.Known || got.X != 1 || got.Y != -1 || got.Intensity != 1 || math.Abs(got.DirectionX-1/math.Sqrt2) > 1e-12 || !got.Right.Down {
		t.Fatalf("diagonal = %+v", got)
	}
	applyPending(t, r, 0, Event{Type: EV_KEY, Code: 31, Value: 1}, Event{Type: EV_KEY, Code: 30, Value: 1})
	opposed := reportFrame(t, r, 0, 4)
	got = ProjectStick(opposed, binding, source)
	if !got.Known || got.X != 0 || got.Y != 0 || got.Intensity != 0 || got.DirectionX != 0 || got.DirectionY != 0 || !got.Up.Down || !got.Right.Down || !got.Down.Down || !got.Left.Down {
		t.Fatalf("opposed = %+v", got)
	}
	bad := source
	bad.Keyboard.Up = -1
	if got := ProjectStick(opposed, binding, bad); got.Known || got.Up.Known || !got.Left.Known {
		t.Fatalf("bad node = %+v", got)
	}
	if got := ProjectStick(opposed, profile.StickBinding{}, source); got.Known {
		t.Fatal("unknown mode projected")
	}
	if got := ProjectStick(r.stop(), binding, source); got.Known || got.Up.Known || got.Right.Known {
		t.Fatalf("terminal = %+v", got)
	}
	if got := ProjectStick(partial, binding, source); !got.Up.Known || got.Down.Known {
		t.Fatal("retained segment changed")
	}
}
