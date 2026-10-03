package input

import (
	"maps"
	"time"
)

type reducer struct {
	latest      Snapshot
	counter     uint8
	seenCounter bool
}

func newReducer(generations Generations) *reducer {
	return &reducer{latest: Snapshot{Connected: true, Generations: generations, Availability: Unconfirmed}}
}
func (r *reducer) snapshot() *Snapshot { s := r.latest; return &s }
func (r *reducer) invalidate(reason string) *Snapshot {
	r.latest.controls = nil
	r.latest.observation = nil
	r.latest.ReadAt = time.Time{}
	r.latest.InvalidationCount++
	r.latest.Availability = Unconfirmed
	r.latest.Reason = reason
	r.latest.Sequence++
	r.seenCounter = false
	return r.snapshot()
}
func (r *reducer) apply(event PhysicalEvent) *Snapshot {
	// Counter wrap/reset is unqualified. Even a plausible step is not proof of
	// continuous delivery. Any other step conservatively discards older knowledge.
	suspicious := r.seenCounter && (r.counter == 255 || event.counter != r.counter+1)
	controls := maps.Clone(r.latest.controls)
	reason := ""
	if suspicious {
		controls = nil
		reason = ERR_INPUT_DROPPED
		r.latest.InvalidationCount++
	}
	if controls == nil {
		controls = make(map[string]PhysicalState)
	}
	controls[event.RegionID] = PhysicalState{Down: event.Down, Known: true}
	r.counter, r.seenCounter = event.counter, true
	r.latest.controls = controls
	r.latest.observation = &event
	r.latest.Availability = Observed
	r.latest.Reason = reason
	r.latest.Sequence++
	r.latest.EventCount++
	return r.snapshot()
}
func (r *reducer) stop() *Snapshot {
	r.latest.controls = nil
	r.latest.observation = nil
	r.latest.ReadAt = time.Time{}
	r.latest.Connected = false
	r.latest.Availability = Unavailable
	r.latest.Reason = ERR_INPUT_READ
	r.latest.Sequence++
	return r.snapshot()
}
