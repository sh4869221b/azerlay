package live

import (
	"context"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/control"
	"github.com/sh4869221b/azerlay/internal/input"
	"github.com/sh4869221b/azerlay/internal/renderer"
)

func TestRates(t *testing.T) {
	start := time.Unix(100, 0)
	now := start
	m := newMetrics(func() time.Time { return now })
	view := &View{Input: input.ManagedSnapshot{Snapshot: &input.Snapshot{}}}
	m.publish(view, nil)
	now = start.Add(250 * time.Millisecond)
	view.Input.Snapshot.EventCount = 3
	m.publish(view, nil)
	m.Draw(nil)
	events, draws := m.Rates()
	if events != 12 || draws != 4 {
		t.Fatalf("initial rates %g/%g", events, draws)
	}
	now = start.Add(time.Second)
	events, draws = m.Rates()
	if events != 3 || draws != 1 {
		t.Fatalf("completed window %g/%g", events, draws)
	}
	now = start.Add(1200 * time.Millisecond)
	view.Input.Snapshot.EventCount = 13
	m.publish(view, nil)
	if events, _ = m.Rates(); events != 3 {
		t.Fatalf("current backlog leaked into completed window: %g", events)
	}
	now = start.Add(2 * time.Second)
	if events, draws = m.Rates(); events != 10 || draws != 0 {
		t.Fatalf("backlog %g/%g", events, draws)
	}
	now = start.Add(4 * time.Second)
	if events, draws = m.Rates(); events != 0 || draws != 0 {
		t.Fatalf("idle %g/%g", events, draws)
	}
}

func TestLatency(t *testing.T) {
	start := time.Unix(100, 0)
	now := start
	m := newMetrics(func() time.Time { return now })
	m.EnableMeasurement(2)
	state := func(seq uint64, snapshot *renderer.OverlaySnapshot) *View {
		return &View{Visible: true, Generation: seq + 1, Generations: Generations{Device: 1, Profile: 1}, Render: snapshot, Input: input.ManagedSnapshot{Snapshot: &input.Snapshot{Sequence: seq, EventCount: seq, ReadAt: now}}}
	}
	first, second, third := &renderer.OverlaySnapshot{}, &renderer.OverlaySnapshot{}, &renderer.OverlaySnapshot{}
	a := state(0, first)
	m.publish(a, nil)
	now = start.Add(time.Millisecond)
	b := state(1, second)
	m.publish(b, a)
	now = start.Add(2 * time.Millisecond)
	c := state(2, third)
	m.publish(c, b)
	m.Draw(second)
	now = start.Add(5 * time.Millisecond)
	m.Draw(third)
	m.Draw(third)
	d := state(3, third)
	m.publish(d, c)
	e := state(4, first)
	e.Generations.Config = 2
	m.publish(e, d)
	m.Draw(first)
	result := m.Measurement()
	if len(result.Samples) != 1 || result.Samples[0].Sequence != 2 || result.Samples[0].Generation != 3 || result.Samples[0].Duration != 3*time.Millisecond {
		t.Fatalf("corresponding samples: %+v", result)
	}
	for _, reason := range []string{"superseded", "stale-draw", "no-semantic-change", "state-change"} {
		if result.Excluded[reason] != 1 {
			t.Fatalf("%s: %+v", reason, result.Excluded)
		}
	}
	if result.Excluded["unmeasured-draw"] != 2 {
		t.Fatalf("redraw exclusions: %+v", result.Excluded)
	}
}

func TestLiveStatus(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	sources, _, _, raw := coordinatorFixture(t)
	raw.value.Store(&input.Snapshot{Connected: true, Availability: input.Unconfirmed, Generations: input.Generations{Device: 1}, InvalidationCount: 4})
	c := Start(context.Background(), sources)
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	view := awaitView(t, c, func(*View) bool { return true })
	status := c.Status()
	if status.EventRate == nil || status.RenderRate != nil || status.DroppedCount == nil || *status.DroppedCount != 4 {
		t.Fatalf("metrics: %+v", status)
	}
	if len(status.Diagnostics) != 1 || status.Diagnostics[0].Code != "INPUT_UNCONFIRMED" {
		t.Fatalf("diagnostics: %+v", status.Diagnostics)
	}
	for range 10 {
		c.Status()
	}
	if c.Latest() != view {
		t.Fatal("status polling published a runtime state")
	}
	select {
	case <-c.Changes():
	default:
	}
	select {
	case <-c.Changes():
		t.Fatal("status polling requested redraw")
	default:
	}
	c.latest.Store(&View{Input: input.ManagedSnapshot{Snapshot: &input.Snapshot{}}, Profile: &control.ProfileSnapshot{}})
	status = c.Status()
	if status.Device != nil || len(status.Diagnostics) != 2 {
		t.Fatalf("unavailable: %+v", status)
	}
}

func TestLatencyRedundantReport(t *testing.T) {
	start := time.Unix(100, 0)
	now := start
	m := newMetrics(func() time.Time { return now })
	m.EnableMeasurement(10)
	a := &View{Visible: true, Render: &renderer.OverlaySnapshot{}, Input: input.ManagedSnapshot{Snapshot: &input.Snapshot{}}}
	m.publish(a, nil)
	now = start.Add(time.Millisecond)
	b := &View{Visible: true, Generation: 2, Render: &renderer.OverlaySnapshot{}, Input: input.ManagedSnapshot{Snapshot: &input.Snapshot{Sequence: 1, EventCount: 1, ReadAt: now}}}
	m.publish(b, a)
	now = start.Add(3 * time.Millisecond)
	c := &View{Visible: true, Generation: 2, Render: b.Render, Input: input.ManagedSnapshot{Snapshot: &input.Snapshot{Sequence: 2, EventCount: 2, ReadAt: now}}}
	m.publish(c, b)
	now = start.Add(5 * time.Millisecond)
	m.Draw(c.Render)
	result := m.Measurement()
	if len(result.Samples) != 1 || result.Samples[0].Sequence != 1 || result.Samples[0].Duration != 4*time.Millisecond || result.Excluded["no-semantic-change"] != 1 {
		t.Fatalf("original semantic read lost or retagged: %+v", result)
	}
}

func TestLatencyDrawDoesNotWait(t *testing.T) {
	m := newMetrics(func() time.Time { return time.Unix(100, 0) })
	m.EnableMeasurement(1)
	m.mu.Lock()
	done := make(chan struct{})
	go func() { m.Draw(nil); close(done) }()
	select {
	case <-done:
		m.mu.Unlock()
	case <-time.After(time.Second):
		m.mu.Unlock()
		<-done
		t.Fatal("GTK draw waited for the status/collector lock")
	}
}

func TestLatencyHiddenRevealExcluded(t *testing.T) {
	now := time.Unix(100, 0)
	m := newMetrics(func() time.Time { return now })
	m.EnableMeasurement(5)
	a := &View{Render: &renderer.OverlaySnapshot{}, Input: input.ManagedSnapshot{Snapshot: &input.Snapshot{}}}
	m.publish(a, nil)
	b := &View{Render: &renderer.OverlaySnapshot{}, Input: input.ManagedSnapshot{Snapshot: &input.Snapshot{Sequence: 1, EventCount: 1, ReadAt: now}}}
	m.publish(b, a)
	c := *b
	c.Visible = true
	m.publish(&c, b)
	m.Draw(c.Render)
	result := m.Measurement()
	if len(result.Samples) != 0 || result.Excluded["state-change"] != 1 {
		t.Fatalf("hidden input measured on reveal: %+v", result)
	}
}

func TestLiveStatusDiagnosticsGeneration(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	sources, _, _, raw := coordinatorFixture(t)
	initial := &input.Snapshot{Connected: true, Availability: input.Unconfirmed, Generations: input.Generations{Device: 1}}
	raw.value.Store(initial)
	c := Start(t.Context(), sources)
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	before := awaitView(t, c, func(*View) bool { return true })
	invalidation := *initial
	invalidation.Sequence, invalidation.InvalidationCount, invalidation.Reason = 1, 1, input.ERR_INPUT_DROPPED
	raw.value.Store(&invalidation)
	raw.changes <- struct{}{}
	changed := awaitView(t, c, func(view *View) bool { return view.Input.Snapshot.Sequence == 1 })
	if changed.Render != before.Render || changed.Generation <= before.Generation {
		t.Fatalf("reason-only change: %d -> %d, same render=%t", before.Generation, changed.Generation, changed.Render == before.Render)
	}
	status := c.Status()
	if len(status.Diagnostics) != 2 || status.Diagnostics[1].Code != input.ERR_INPUT_DROPPED {
		t.Fatalf("actual dropped diagnostic: %+v", status.Diagnostics)
	}
	counters := invalidation
	counters.Sequence, counters.EventCount = 2, 1
	raw.value.Store(&counters)
	raw.changes <- struct{}{}
	unchanged := awaitView(t, c, func(view *View) bool { return view.Input.Snapshot.Sequence == 2 })
	if unchanged.Generation != changed.Generation || unchanged.Render != changed.Render {
		t.Fatal("counters-only publication changed semantic generation")
	}
}
