package input

import (
	"slices"

	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/matching"
)

type PhysicalControl struct {
	RegionID    string
	SourceID    int
	Down, Known bool
}
type MatchingState struct {
	Connected          bool
	Sequence           uint64
	Generations        Generations
	ScopeSupported     bool
	HasUnknownBindings bool
	Controls           []matching.Control
	Outputs            []matching.Output
	Physical           []PhysicalControl
}

// ProjectMatching keeps static assignment candidates separate from physical
// observations, including when no static profile assignment can be mapped.
func ProjectMatching(snapshot *Snapshot, index matching.Index, definition layout.Definition) MatchingState {
	state := MatchingState{Connected: snapshot.Connected, Sequence: snapshot.Sequence, Generations: snapshot.Generations, HasUnknownBindings: index.HasUnknownBindings}
	for _, control := range index.Controls {
		control.Bindings = slices.Clone(control.Bindings)
		state.Controls = append(state.Controls, control)
	}
	for _, output := range index.Outputs {
		output.Candidates = slices.Clone(output.Candidates)
		state.Outputs = append(state.Outputs, output)
	}
	state.ScopeSupported = definition.Model == "cyborg-ii" && definition.Hand == "left"
	if !state.ScopeSupported {
		return state
	}
	for _, control := range definition.Controls {
		if control.ID == "stick.main" {
			continue
		}
		physical := PhysicalControl{RegionID: control.ID, SourceID: control.SourceInputID}
		if state.ScopeSupported && snapshot.Connected {
			observed := snapshot.Physical(control.ID)
			physical.Down, physical.Known = observed.Down, observed.Known
		}
		state.Physical = append(state.Physical, physical)
	}
	return state
}
