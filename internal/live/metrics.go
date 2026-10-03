package live

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/sh4869221b/azerlay/internal/renderer"
)

type rates struct {
	start                  time.Time
	window                 int64
	events, previousEvents uint64
}

func (r *rates) rotate(now time.Time) {
	window := int64(now.Sub(r.start) / time.Second)
	if window > r.window {
		r.previousEvents = 0
		if window == r.window+1 {
			r.previousEvents = r.events
		}
		r.events, r.window = 0, window
	}
}
func (r *rates) read(now time.Time) float64 {
	r.rotate(now)
	if r.window > 0 {
		return float64(r.previousEvents)
	}
	if elapsed := now.Sub(r.start).Seconds(); elapsed > 0 {
		return float64(r.events) / elapsed
	}
	return 0
}

type TimingSample struct {
	Sequence, Generation uint64
	Read, Draw           time.Duration
	Duration             time.Duration
}
type Measurement struct {
	Samples  []TimingSample
	Excluded map[string]uint64
}
type frameTiming struct {
	snapshot             *renderer.OverlaySnapshot
	sequence, generation uint64
	read                 time.Time
}
type Metrics struct {
	mu            sync.Mutex
	now           func() time.Time
	rates         rates
	events        uint64
	pending       atomic.Pointer[frameTiming]
	collector     atomic.Pointer[measurementCollector]
	draws         atomic.Pointer[drawCounts]
	eventMeasured atomic.Bool
}

type drawCounts struct {
	window            int64
	current, previous uint64
}
type measurementCollector struct {
	captures chan TimingSample
	samples  []TimingSample
	count    atomic.Uint64
	limit    uint64
	excluded map[string]*atomic.Uint64
}

func NewMetrics() *Metrics { return newMetrics(time.Now) }
func newMetrics(now func() time.Time) *Metrics {
	return &Metrics{now: now, rates: rates{start: now()}}
}

func (m *Metrics) EnableMeasurement(limit int) {
	collector := &measurementCollector{captures: make(chan TimingSample, limit), limit: uint64(limit), excluded: make(map[string]*atomic.Uint64)}
	for _, reason := range []string{"superseded", "state-change", "no-semantic-change", "unmeasured-draw", "stale-draw", "collector-full"} {
		collector.excluded[reason] = &atomic.Uint64{}
	}
	m.collector.Store(collector)
}
func (m *Metrics) Measurement() Measurement {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := Measurement{Excluded: make(map[string]uint64)}
	if collector := m.collector.Load(); collector != nil {
		for draining := true; draining; {
			select {
			case sample := <-collector.captures:
				collector.samples = append(collector.samples, sample)
			default:
				draining = false
			}
		}
		result.Samples = append([]TimingSample(nil), collector.samples...)
		for reason, count := range collector.excluded {
			result.Excluded[reason] = count.Load()
		}
	}
	return result
}
func (m *Metrics) exclude(reason string, count uint64) {
	if collector := m.collector.Load(); collector != nil {
		collector.excluded[reason].Add(count)
	}
}
func (m *Metrics) publish(view, previous *View) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rates.rotate(m.now())
	raw := view.Input.Snapshot
	if raw.Connected || raw.EventCount > 0 {
		m.eventMeasured.Store(true)
	}
	delta := raw.EventCount - m.events
	m.rates.events += delta
	m.events = raw.EventCount
	if delta == 0 {
		if pending := m.pending.Load(); pending != nil && (view.Render != pending.snapshot || previous != nil && (view.Visible != previous.Visible || view.Generations != previous.Generations || raw.InvalidationCount != previous.Input.Snapshot.InvalidationCount)) {
			if m.pending.CompareAndSwap(pending, nil) {
				m.exclude("superseded", 1)
			}
		}
		return
	}
	if delta > 1 {
		m.exclude("superseded", delta-1)
	}
	if previous == nil || !view.Visible || view.Visible != previous.Visible || view.Generations != previous.Generations || raw.InvalidationCount != previous.Input.Snapshot.InvalidationCount || raw.ReadAt.IsZero() {
		if m.pending.Swap(nil) != nil {
			m.exclude("superseded", 1)
		}
		m.exclude("state-change", 1)
		return
	}
	if view.Render == previous.Render {
		m.exclude("no-semantic-change", 1)
		return
	}
	if m.pending.Swap(&frameTiming{snapshot: view.Render, sequence: raw.Sequence, generation: view.Generation, read: raw.ReadAt}) != nil {
		m.exclude("superseded", 1)
	}
}

// Draw has a single writer on the GTK owner thread and never waits for readers.
func (m *Metrics) Draw(snapshot *renderer.OverlaySnapshot) {
	now := m.now()
	window := int64(now.Sub(m.rates.start) / time.Second)
	draws := drawCounts{window: window, current: 1}
	if previous := m.draws.Load(); previous != nil {
		if previous.window == window {
			draws.current, draws.previous = previous.current+1, previous.previous
		} else if previous.window+1 == window {
			draws.previous = previous.current
		}
	}
	m.draws.Store(&draws)
	tag := m.pending.Load()
	if tag == nil {
		m.exclude("unmeasured-draw", 1)
		return
	}
	if tag.snapshot != snapshot || !m.pending.CompareAndSwap(tag, nil) {
		m.exclude("stale-draw", 1)
		return
	}
	if collector := m.collector.Load(); collector != nil {
		if collector.count.Add(1) <= collector.limit {
			select {
			case collector.captures <- TimingSample{Sequence: tag.sequence, Generation: tag.generation, Read: tag.read.Sub(m.rates.start), Draw: now.Sub(m.rates.start), Duration: now.Sub(tag.read)}:
			default:
				m.exclude("collector-full", 1)
			}
		} else {
			m.exclude("collector-full", 1)
		}
	}
}
func (m *Metrics) Rates() (float64, float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	events := m.rates.read(now)
	var renderRate float64
	if draws := m.draws.Load(); draws != nil {
		window := int64(now.Sub(m.rates.start) / time.Second)
		if window == 0 && now.After(m.rates.start) {
			renderRate = float64(draws.current) / now.Sub(m.rates.start).Seconds()
		} else if draws.window == window {
			renderRate = float64(draws.previous)
		} else if draws.window+1 == window {
			renderRate = float64(draws.current)
		}
	}
	return events, renderRate
}

func (m *Metrics) Measured() (bool, bool) { return m.eventMeasured.Load(), m.draws.Load() != nil }
