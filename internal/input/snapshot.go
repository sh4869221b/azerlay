package input

import (
	"maps"
	"time"
)

type Generations struct{ Device, Profile uint64 }
type Availability string

const (
	Unconfirmed Availability = "unconfirmed"
	Observed    Availability = "observed"
	Unavailable Availability = "unavailable"
)

type PhysicalState struct{ Down, Known bool }

// Snapshot is the last received observation, not guaranteed current state.
// Undetected terminal-report loss can leave an observation stale indefinitely.
type Snapshot struct {
	Sequence          uint64
	Connected         bool
	Generations       Generations
	Availability      Availability
	Reason            string
	ReadAt            time.Time
	EventCount        uint64
	InvalidationCount uint64
	controls          map[string]PhysicalState
	observation       *PhysicalEvent
}

func (s *Snapshot) Physical(region string) PhysicalState { return s.controls[region] }
func (s *Snapshot) Controls() map[string]PhysicalState   { return maps.Clone(s.controls) }

// LastObservation identifies the report that produced this publication.
// Initial, invalidated and disconnected snapshots have no observation.
func (s *Snapshot) LastObservation() (PhysicalEvent, bool) {
	if s.observation == nil {
		return PhysicalEvent{}, false
	}
	return *s.observation, true
}
