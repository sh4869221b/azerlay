package input

import (
	"reflect"
	"testing"

	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/matching"
	"github.com/sh4869221b/azerlay/internal/profile"
)

func matchingTestIndex(t *testing.T) matching.Index {
	t.Helper()
	definition, err := layout.LoadEmbedded("cyborg-ii", "left")
	if err != nil {
		t.Fatal(err)
	}
	id := func(value int) *int { return &value }
	keyboard := func(code profile.CanonicalCode) profile.TriggerBinding {
		return profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Actions: []profile.Action{{Kind: profile.ActionKeyboard, Code: code}}}
	}
	selected := profile.Profile{Controls: []profile.ControlBinding{
		{SourceIdentity: profile.SourceIdentity{InputID: id(4)}, Bindings: []profile.TriggerBinding{keyboard(profile.KEY_U)}},
		{SourceIdentity: profile.SourceIdentity{InputID: id(8)}, Bindings: []profile.TriggerBinding{keyboard(profile.KEY_U)}},
		{SourceIdentity: profile.SourceIdentity{InputID: id(12)}, Bindings: []profile.TriggerBinding{keyboard(profile.KEY_P)}},
	}}
	return matching.Build(selected, profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export"}, definition,
		matching.Context{Model: definition.Model, Hand: definition.Hand, Applicability: definition.Applicability})
}

func matchingOutput(t *testing.T, state MatchingState, code profile.CanonicalCode) MatchingOutput {
	t.Helper()
	for _, output := range state.Outputs {
		if output.Code == code {
			return output
		}
	}
	t.Fatalf("missing output %s", code)
	return MatchingOutput{}
}

func TestMatchingProjection(t *testing.T) {
	t.Parallel()
	index := matchingTestIndex(t)
	r := newReducer(2, Generations{Device: 3, Profile: 7})
	sources := map[profile.CanonicalCode]int{profile.KEY_U: 0, profile.KEY_P: 0}
	project := func(snapshot *Snapshot) MatchingState { return ProjectMatching(snapshot, index, sources) }
	initial := project(r.snapshot())
	if !initial.Connected || initial.Sequence != 0 || initial.Generations != (Generations{Device: 3, Profile: 7}) || matchingOutput(t, initial, profile.KEY_U).Known {
		t.Fatalf("initial unobserved state = %#v", initial)
	}
	applyPending(t, r, 1, Event{Type: EV_KEY, Code: 22, Value: 1})
	if got := project(r.snapshot()); !reflect.DeepEqual(got, initial) {
		t.Fatal("pending frame changed projection")
	}
	other := reportFrame(t, r, 1, 1)
	if matchingOutput(t, project(other), profile.KEY_U).Known {
		t.Fatal("other node's same key was accepted")
	}
	applyPending(t, r, 0, Event{Type: EV_KEY, Code: 22, Value: 1}, Event{Type: EV_KEY, Code: 25, Value: 0})
	if matchingOutput(t, project(r.snapshot()), profile.KEY_U).Known {
		t.Fatal("unreported selected frame was accepted")
	}
	pressed := reportFrame(t, r, 0, 2)
	pressedState := project(pressed)
	u := matchingOutput(t, pressedState, profile.KEY_U)
	p := matchingOutput(t, pressedState, profile.KEY_P)
	if !u.Known || !u.Down || !u.Ambiguous || len(u.Candidates) != 2 || !p.Known || p.Down || p.Ambiguous || pressedState.Sequence != 2 {
		t.Fatalf("committed projection = %#v", pressedState)
	}
	missing := ProjectMatching(pressed, index, map[profile.CanonicalCode]int{profile.KEY_U: 0})
	if matchingOutput(t, missing, profile.KEY_P).Known {
		t.Fatal("missing selector became node zero")
	}
	invalid := ProjectMatching(pressed, index, map[profile.CanonicalCode]int{profile.KEY_U: -1, profile.KEY_P: 2})
	if matchingOutput(t, invalid, profile.KEY_U).Known || matchingOutput(t, invalid, profile.KEY_P).Known {
		t.Fatal("invalid node selector was accepted")
	}

	pressedState.Controls[0].Bindings[0].Kind = profile.BindingUnknown
	pressedState.Outputs[0].Candidates[0].RegionID = "changed"
	if index.Controls[0].Bindings[0].Kind != profile.BindingKeyboard || index.Outputs[0].Candidates[0].RegionID == "changed" {
		t.Fatal("projection aliases static index")
	}
	applyPending(t, r, 0, Event{Type: EV_KEY, Code: 22, Value: 0})
	released := reportFrame(t, r, 0, 3)
	if value := matchingOutput(t, project(released), profile.KEY_U); !value.Known || value.Down {
		t.Fatalf("release = %#v", value)
	}
	if value := matchingOutput(t, project(pressed), profile.KEY_U); !value.Known || !value.Down {
		t.Fatalf("retained snapshot changed = %#v", value)
	}
	dropped, err := r.apply(0, Event{Type: EV_SYN, Code: SYN_DROPPED})
	if err != nil || dropped == nil {
		t.Fatalf("drop = %v", err)
	}
	if value := matchingOutput(t, project(dropped), profile.KEY_U); value.Known || value.Down {
		t.Fatalf("dropped observation = %#v", value)
	}
	if value := matchingOutput(t, project(pressed), profile.KEY_U); !value.Known || !value.Down {
		t.Fatalf("drop changed retained snapshot = %#v", value)
	}
	terminal := project(r.stop())
	if terminal.Connected || terminal.Sequence != 5 || terminal.Generations != initial.Generations || matchingOutput(t, terminal, profile.KEY_U).Known || matchingOutput(t, terminal, profile.KEY_P).Known {
		t.Fatalf("terminal state = %#v", terminal)
	}
}

func TestMatchingProjectionUnknownCode(t *testing.T) {
	t.Parallel()
	index := matchingTestIndex(t)
	index.Outputs = append(index.Outputs, matching.Output{Code: "KEY_FUTURE"})
	r := newReducer(1, Generations{})
	applyPending(t, r, 0, Event{Type: EV_KEY, Code: 0, Value: 1})
	snapshot := reportFrame(t, r, 0, 1)
	state := ProjectMatching(snapshot, index, map[profile.CanonicalCode]int{"KEY_FUTURE": 0})
	if value := matchingOutput(t, state, "KEY_FUTURE"); value.Known || value.Down {
		t.Fatalf("unknown canonical code aliased raw zero: %#v", value)
	}
}
