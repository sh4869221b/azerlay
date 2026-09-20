package input

import "maps"

// reducer has one owner. Pending events cannot affect committed state until
// their own node reports, regardless of other nodes' report boundaries.
type reducer struct {
	nodes   []nodeState
	pending [][]Event
	latest  Snapshot
}

func newReducer(nodeCount int, generations Generations) *reducer {
	nodes := make([]nodeState, nodeCount)
	for i := range nodes {
		nodes[i] = nodeState{keys: make(map[uint16]bool), absolute: make(map[uint16]int32)}
	}
	return &reducer{
		nodes: nodes, pending: make([][]Event, nodeCount),
		latest: Snapshot{Connected: true, Generations: generations, reportingNode: -1},
	}
}

// snapshot copies the public header; private storage is never mutated after
// publication, so readers can safely retain it across subsequent reductions.
func (r *reducer) snapshot() *Snapshot {
	snapshot := r.latest
	return &snapshot
}

// apply consumes a session-assigned node index in merged receive order.
// A nil snapshot means no publication. The caller must stop on an error.
func (r *reducer) apply(node int, event Event) (*Snapshot, error) {
	if !r.latest.Connected {
		return nil, nil
	}
	switch event.Type {
	case EV_SYN:
		switch event.Code {
		case SYN_REPORT:
			return r.commit(node, event), nil
		case SYN_DROPPED:
			return nil, &InputError{Code: ERR_INPUT_DROPPED}
		default:
			return nil, nil
		}
	case EV_KEY:
		if event.Value < int32(KeyRelease) || event.Value > int32(KeyRepeat) {
			return nil, &InputError{Code: ERR_INPUT_EVENT}
		}
		if event.Code > 0x2ff {
			return nil, nil
		}
	case EV_ABS:
		if event.Code > 0x3f {
			return nil, nil
		}
	case EV_REL:
		if event.Code > 0x0c || event.Code == 0x0a {
			return nil, nil
		}
	case EV_MSC:
		if event.Code != MSC_SCAN {
			return nil, nil
		}
	default:
		return nil, nil
	}
	r.pending[node] = append(r.pending[node], event)
	return nil, nil
}

func (r *reducer) commit(node int, report Event) *Snapshot {
	frame := r.pending[node]
	r.pending[node] = nil
	relative := make(map[uint16]int64)
	timestamp := max(r.latest.Timestamp, report.Timestamp)
	for _, event := range frame {
		timestamp = max(timestamp, event.Timestamp)
		switch event.Type {
		case EV_KEY:
			r.nodes[node].keys[event.Code] = event.Value != int32(KeyRelease)
		case EV_ABS:
			r.nodes[node].absolute[event.Code] = event.Value
		case EV_REL:
			relative[event.Code] += int64(event.Value)
		}
	}
	nodes := make([]nodeState, len(r.nodes))
	for i, state := range r.nodes {
		nodes[i] = nodeState{keys: maps.Clone(state.keys), absolute: maps.Clone(state.absolute)}
	}
	r.latest = Snapshot{
		Sequence: r.latest.Sequence + 1, Timestamp: timestamp,
		Connected: true, Generations: r.latest.Generations,
		nodes: nodes, reportingNode: node, events: frame, relative: relative,
	}
	return r.snapshot()
}

// stop discards every pending frame and publishes a single cleared terminal
// state. Further stop/apply calls cannot produce publications.
func (r *reducer) stop() *Snapshot {
	if !r.latest.Connected {
		return nil
	}
	r.nodes = nil
	r.pending = nil
	r.latest = Snapshot{
		Sequence: r.latest.Sequence + 1, Timestamp: r.latest.Timestamp,
		Generations: r.latest.Generations, reportingNode: -1,
	}
	return r.snapshot()
}
