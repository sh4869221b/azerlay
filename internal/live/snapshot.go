package live

import (
	"reflect"
	"slices"

	"github.com/sh4869221b/azerlay/internal/control"
	"github.com/sh4869221b/azerlay/internal/gameprofile"
	"github.com/sh4869221b/azerlay/internal/input"
	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/matching"
	"github.com/sh4869221b/azerlay/internal/renderer"
)

// Assembly caches static assignments and retains the renderer pointer when only
// input counters or publication sequence change. It belongs to the coordinator.
type Assembly struct {
	profile                            *control.ProfileSnapshot
	layoutGeneration, labelsGeneration uint64
	assignments                        map[string]renderer.Control
	index                              matching.Index
	content                            renderer.Content
	snapshot                           *renderer.OverlaySnapshot
}

type AssemblyState struct {
	Profile                            *control.ProfileSnapshot
	Definition                         layout.Definition
	LayoutGeneration, LabelsGeneration uint64
	Game                               gameprofile.Definition
	Input                              *input.Snapshot
	ReloadFailed                       bool
}

func (a *Assembly) Build(state AssemblyState) *renderer.OverlaySnapshot {
	layoutChanged := a.layoutGeneration != state.LayoutGeneration
	staticChanged := a.profile != state.Profile || a.layoutGeneration != state.LayoutGeneration || a.labelsGeneration != state.LabelsGeneration
	if staticChanged {
		a.profile, a.layoutGeneration, a.labelsGeneration = state.Profile, state.LayoutGeneration, state.LabelsGeneration
		a.assignments = make(map[string]renderer.Control)
		a.index = matching.Index{}
		if state.Profile.Profile != nil {
			selected := *state.Profile.Profile
			a.index = matching.Build(selected, state.Profile.Source, state.Definition)
			ambiguous := make(map[[2]int]bool)
			for _, output := range a.index.Outputs {
				if output.Ambiguous {
					for _, candidate := range output.Candidates {
						ambiguous[[2]int{candidate.ControlIndex, candidate.BindingIndex}] = true
					}
				}
			}
			for _, mapped := range a.index.Controls {
				if mapped.MappingState != matching.MappingMapped {
					continue
				}
				control := selected.Controls[mapped.ControlIndex]
				value := a.assignments[mapped.RegionID]
				for i, binding := range control.Bindings {
					resolved := gameprofile.Resolve(control, binding, state.Game)
					value.Assignments = append(value.Assignments, renderer.Assignment{Trigger: binding.Trigger, Kind: binding.Kind, Label: resolved.Label, BindingDisplay: resolved.BindingDisplay, Ambiguous: ambiguous[[2]int{mapped.ControlIndex, i}]})
				}
				a.assignments[mapped.RegionID] = value
			}
		}
	}
	content := renderer.Content{Controls: make(map[string]renderer.Control, len(state.Definition.Controls))}
	for region, assignment := range a.assignments {
		assignment.Assignments = slices.Clone(assignment.Assignments)
		content.Controls[region] = assignment
	}
	physical := input.ProjectMatching(state.Input, a.index, state.Definition)
	for _, observation := range physical.Physical {
		value := content.Controls[observation.RegionID]
		value.Known, value.Down = observation.Known, observation.Down
		content.Controls[observation.RegionID] = value
	}
	if state.Profile.Profile == nil {
		content.Status = renderer.StatusProfileMissing
	} else if state.Profile.Profile.Name != nil {
		content.ProfileName = *state.Profile.Profile.Name
	}
	if content.Status == renderer.StatusNone && !state.Input.Connected {
		content.Status = renderer.StatusDisconnected
	}
	if state.ReloadFailed {
		content.Status = renderer.StatusReloadFailed
	}
	if a.snapshot == nil || layoutChanged || !reflect.DeepEqual(a.content, content) {
		a.content = content
		a.snapshot = renderer.NewSnapshot(state.Definition, content)
	}
	return a.snapshot
}
