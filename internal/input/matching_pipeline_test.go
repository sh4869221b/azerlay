//go:build linux

package input

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/matching"
	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profilesource"
)

type matchingVisualControl struct {
	Index    int      `json:"index"`
	Region   string   `json:"region"`
	Mapping  string   `json:"mapping"`
	Bindings []string `json:"bindings"`
}

type matchingVisualOutput struct {
	Code       profile.CanonicalCode `json:"code"`
	Candidates []string              `json:"candidates"`
	Ambiguous  bool                  `json:"ambiguous"`
	Unresolved bool                  `json:"unresolved"`
}

type matchingVisualKey struct {
	Code  profile.CanonicalCode `json:"code"`
	Known bool                  `json:"known"`
	Down  bool                  `json:"down"`
}

type matchingVisualFrame struct {
	Name        string              `json:"name"`
	Connected   bool                `json:"connected"`
	Sequence    uint64              `json:"sequence"`
	Generations Generations         `json:"generations"`
	Keys        []matchingVisualKey `json:"keys"`
}

type matchingVisualGolden struct {
	ScopeSupported     bool                    `json:"scope_supported"`
	HasUnknownBindings bool                    `json:"has_unknown_bindings"`
	Controls           []matchingVisualControl `json:"controls"`
	Outputs            []matchingVisualOutput  `json:"outputs"`
	Frames             []matchingVisualFrame   `json:"frames"`
}

func fixtureControl(t *testing.T, path string, index int) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Inputs []map[string]json.RawMessage `json:"inputs"`
	}
	if err := json.Unmarshal(data, &source); err != nil || index >= len(source.Inputs) {
		t.Fatalf("fixture %s input %d: %v", path, index, err)
	}
	return source.Inputs[index]
}

func cloneFixtureControl(input map[string]json.RawMessage) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage, len(input))
	for key, value := range input {
		result[key] = append(json.RawMessage(nil), value...)
	}
	return result
}

func matchingPipelineProfile(t *testing.T) profilesource.Prepared {
	t.Helper()
	u := fixtureControl(t, "../profileadapter/testdata/single.input.json", 0)
	u["id"], u["pinOne"], u["pinTwo"] = json.RawMessage("4"), json.RawMessage("5"), json.RawMessage("255")
	u2 := cloneFixtureControl(u)
	u2["id"], u2["pinOne"] = json.RawMessage("8"), json.RawMessage("11")
	p := cloneFixtureControl(u)
	p["id"], p["pinOne"] = json.RawMessage("12"), json.RawMessage("17")
	p["keyValues"], p["metaValues"] = json.RawMessage(`["KeyP","0","0","0"]`), json.RawMessage(`["0","0","0"]`)
	turbo := fixtureControl(t, "../profileadapter/testdata/v1-settings.input.json", 0)
	turbo["id"] = json.RawMessage("999")
	unbound := fixtureControl(t, "../profileadapter/testdata/unbound.input.json", 0)
	unbound["id"], unbound["pinOne"] = json.RawMessage("3"), json.RawMessage("4")
	unknown := map[string]json.RawMessage{
		"id": json.RawMessage("2"), "pinOne": json.RawMessage("3"), "pinTwo": json.RawMessage("255"),
		"types": json.RawMessage(`["11","11","11"]`), "future": json.RawMessage("true"),
	}
	encoded, err := json.Marshal(struct {
		ID     string                       `json:"id"`
		Inputs []map[string]json.RawMessage `json:"inputs"`
	}{ID: "synthetic", Inputs: []map[string]json.RawMessage{u, u2, p, turbo, unbound, unknown}})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := profilesource.PrepareText(string(encoded), profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export"})
	if err != nil {
		t.Fatal(err)
	}
	return prepared
}

func appendMatchingVisualFrame(golden *matchingVisualGolden, name string, state MatchingState) {
	frame := matchingVisualFrame{Name: name, Connected: state.Connected, Sequence: state.Sequence, Generations: state.Generations}
	for _, output := range state.Outputs {
		frame.Keys = append(frame.Keys, matchingVisualKey{Code: output.Code, Known: output.Known, Down: output.Down})
	}
	golden.Frames = append(golden.Frames, frame)
}

func TestMatchingVisualStateGolden(t *testing.T) {
	t.Parallel()
	prepared := matchingPipelineProfile(t)
	home := t.TempDir()
	store := profilesource.NewImportedSource(home)
	selection, err := store.Import(context.Background(), prepared, 1, profilesource.Origin{Kind: "text"})
	if err != nil {
		t.Fatal(err)
	}
	selected, err := store.Selected(context.Background())
	if err != nil || selected != selection {
		t.Fatalf("selection = %#v, %v", selected, err)
	}
	loaded, err := profilesource.NewImportedSource(home).Load(context.Background(), selection.Source)
	if err != nil || loaded == nil || !reflect.DeepEqual(*loaded, prepared.Bundle()) {
		t.Fatalf("loaded normalization = %v", err)
	}
	definition, err := layout.LoadEmbedded("cyborg-ii", "left")
	if err != nil {
		t.Fatal(err)
	}
	index := matching.Build(loaded.Profiles[selection.ProfileIndex-1], loaded.Source, definition,
		matching.Context{Model: definition.Model, Hand: definition.Hand, Applicability: definition.Applicability})
	sources := map[profile.CanonicalCode]int{profile.KEY_LEFTCTRL: 0, profile.KEY_P: 0, profile.KEY_T: 0, profile.KEY_U: 0}
	session := newIntegrationSession(t, 2, Generations{Device: 3, Profile: 7})
	project := func(snapshot *Snapshot) MatchingState { return ProjectMatching(snapshot, index, sources) }
	initial := project(session.Latest())
	visual := matchingVisualGolden{ScopeSupported: initial.ScopeSupported, HasUnknownBindings: initial.HasUnknownBindings}
	for _, control := range initial.Controls {
		entry := matchingVisualControl{Index: control.ControlIndex, Region: control.RegionID, Mapping: string(control.MappingState)}
		for _, binding := range control.Bindings {
			entry.Bindings = append(entry.Bindings, string(binding.Trigger)+":"+string(binding.Kind))
		}
		visual.Controls = append(visual.Controls, entry)
	}
	for _, output := range initial.Outputs {
		entry := matchingVisualOutput{Code: output.Code, Ambiguous: output.Ambiguous, Unresolved: output.HasUnresolvedCandidates}
		for _, candidate := range output.Candidates {
			entry.Candidates = append(entry.Candidates, fmt.Sprintf("%d/%d/%s/%s/%s/%s", candidate.ControlIndex, candidate.BindingIndex, candidate.Trigger, candidate.Kind, candidate.RegionID, candidate.MappingState))
		}
		visual.Outputs = append(visual.Outputs, entry)
	}
	appendMatchingVisualFrame(&visual, "initial", initial)
	unique := project(session.send(t, 0, Event{Type: EV_KEY, Code: 25, Value: 1}, Event{Type: EV_SYN}))
	appendMatchingVisualFrame(&visual, "unique_press", unique)
	duplicate := project(session.send(t, 0, Event{Type: EV_KEY, Code: 22, Value: 1}, Event{Type: EV_SYN}))
	appendMatchingVisualFrame(&visual, "duplicate_press", duplicate)
	released := project(session.send(t, 0, Event{Type: EV_KEY, Code: 25}, Event{Type: EV_KEY, Code: 22}, Event{Type: EV_SYN}))
	appendMatchingVisualFrame(&visual, "release", released)
	if err := session.writers[0].Close(); err != nil {
		t.Fatal(err)
	}
	assertIntegrationStopped(t, session, ERR_INPUT_READ, released.Sequence+1, released.Generations)
	appendMatchingVisualFrame(&visual, "disconnect", project(session.Latest()))
	if retained := project(session.Latest()); retained.Connected || retained.Sequence != 4 {
		t.Fatalf("disconnect projection = %#v", retained)
	}
	expectedJSON, err := os.ReadFile("testdata/matching-visual-state.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected matchingVisualGolden
	if err := json.Unmarshal(expectedJSON, &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(visual, expected) {
		actualJSON, err := json.MarshalIndent(visual, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("visual state =\n%s\nwant literal testdata/matching-visual-state.json", actualJSON)
	}
}
