package input

import (
	"slices"
	"time"
)

// Snapshot contains committed node-local state and only the reporting node's
// current frame. Its private storage is immutable; accessors return values or
// defensive copies. Exported scalar fields belong to the receiving caller.
type Snapshot struct {
	Sequence    uint64
	Timestamp   time.Duration
	Connected   bool
	Generations Generations

	nodes         []nodeState
	reportingNode int
	events        []Event
	relative      map[uint16]int64
}

type nodeState struct {
	keys     map[uint16]bool
	absolute map[uint16]int32
}

// Key reports the last observed committed state. known is false until a key is
// observed; an unknown initial state must not be interpreted as released.
func (s *Snapshot) Key(node int, code uint16) (down, known bool) {
	if node < 0 || node >= len(s.nodes) {
		return false, false
	}
	down, known = s.nodes[node].keys[code]
	return down, known
}

// Absolute returns the raw committed value, without normalization. known is
// false until that node's axis is observed.
func (s *Snapshot) Absolute(node int, code uint16) (value int32, known bool) {
	if node < 0 || node >= len(s.nodes) {
		return 0, false
	}
	value, known = s.nodes[node].absolute[code]
	return value, known
}

// ReportingNode identifies the selected-path index whose SYN_REPORT published
// this frame. Initial and terminal snapshots have no reporting node.
func (s *Snapshot) ReportingNode() (node int, present bool) {
	return s.reportingNode, s.reportingNode >= 0
}

// Relative returns the reporting frame's accumulated delta for one raw code.
// Legacy and high-resolution wheel codes remain separate.
func (s *Snapshot) Relative(code uint16) (delta int64, present bool) {
	delta, present = s.relative[code]
	return delta, present
}

// FrameEvents returns a copy of relevant events in the reporting frame's order.
// It contains no SYN marker, other nodes' pending events, or previous frames.
func (s *Snapshot) FrameEvents() []Event { return slices.Clone(s.events) }

// KeyTransitions returns all press, release and repeat transitions in the
// reporting frame, including multiple changes of a single key.
func (s *Snapshot) KeyTransitions() []KeyTransition {
	var transitions []KeyTransition
	for _, event := range s.events {
		if event.Type == EV_KEY {
			transitions = append(transitions, KeyTransition{
				Timestamp: event.Timestamp, Code: event.Code, Action: KeyAction(event.Value),
			})
		}
	}
	return transitions
}
