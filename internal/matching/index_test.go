package matching

import (
	"reflect"
	"testing"

	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/profile"
)

var matchingSource = profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export"}

func matchingLayout(t *testing.T) layout.Definition {
	t.Helper()
	definition, err := layout.LoadEmbedded("cyborg-ii", "left")
	if err != nil {
		t.Fatal(err)
	}
	return definition
}

func matchingID(value int) *int { return &value }

func matchingControl(id *int, binding ...profile.TriggerBinding) profile.ControlBinding {
	return profile.ControlBinding{SourceIdentity: profile.SourceIdentity{InputID: id}, Bindings: binding}
}

func matchingKeyboard(trigger profile.TriggerKind, code profile.CanonicalCode, modifiers ...profile.CanonicalCode) profile.TriggerBinding {
	return profile.TriggerBinding{Trigger: trigger, Kind: profile.BindingKeyboard, Actions: []profile.Action{{Kind: profile.ActionKeyboard, Code: code, Modifiers: modifiers}}}
}

func outputFor(t *testing.T, index Index, code profile.CanonicalCode) Output {
	t.Helper()
	for _, output := range index.Outputs {
		if output.Code == code {
			return output
		}
	}
	t.Fatalf("missing output %s", code)
	return Output{}
}

func TestBuildCandidates(t *testing.T) {
	t.Parallel()
	definition := matchingLayout(t)
	first := matchingControl(matchingID(4), matchingKeyboard(profile.TriggerSingle, profile.KEY_U))
	first.SourceIdentity.PinOne = matchingID(5)
	first.SourceIdentity.PinTwo = matchingID(255)
	unique := Build(profile.Profile{Controls: []profile.ControlBinding{first}}, matchingSource, definition)
	if !unique.ScopeSupported || len(unique.Controls) != 1 || unique.Controls[0].RegionID != "grid.c1.r1" || unique.Controls[0].MappingState != MappingMapped || len(unique.Outputs) != 1 {
		t.Fatalf("unique mapping = %#v", unique)
	}
	u := outputFor(t, unique, profile.KEY_U)
	if len(u.Candidates) != 1 || u.Ambiguous || u.HasUnresolvedCandidates || u.Candidates[0].ControlIndex != 0 || u.Candidates[0].BindingIndex != 0 {
		t.Fatalf("unique candidate = %#v", u)
	}

	second := matchingControl(matchingID(8), matchingKeyboard(profile.TriggerSingle, profile.KEY_U))
	duplicate := Build(profile.Profile{Controls: []profile.ControlBinding{first, second}}, matchingSource, definition)
	u = outputFor(t, duplicate, profile.KEY_U)
	if len(u.Candidates) != 2 || !u.Ambiguous || u.HasUnresolvedCandidates || u.Candidates[0].ControlIndex != 0 || u.Candidates[1].ControlIndex != 1 {
		t.Fatalf("duplicate candidates = %#v", u)
	}
	if duplicate.Controls[1].RegionID != "grid.c2.r1" {
		t.Fatalf("second region = %#v", duplicate.Controls[1])
	}

	reversed := Build(profile.Profile{Controls: []profile.ControlBinding{second, first}}, matchingSource, definition)
	r := outputFor(t, reversed, profile.KEY_U)
	if len(r.Candidates) != 2 || !r.Ambiguous || r.Candidates[0].RegionID != "grid.c2.r1" || r.Candidates[1].RegionID != "grid.c1.r1" {
		t.Fatalf("source order changed candidate coverage: %#v", r)
	}

	multi := matchingControl(matchingID(4), matchingKeyboard(profile.TriggerSingle, profile.KEY_U), matchingKeyboard(profile.TriggerLong, profile.KEY_U), matchingKeyboard(profile.TriggerDouble, profile.KEY_P))
	multipleTriggers := Build(profile.Profile{Controls: []profile.ControlBinding{multi}}, matchingSource, definition)
	u = outputFor(t, multipleTriggers, profile.KEY_U)
	if len(u.Candidates) != 2 || u.Ambiguous || u.Candidates[0].BindingIndex != 0 || u.Candidates[1].BindingIndex != 1 || u.Candidates[1].Trigger != profile.TriggerLong {
		t.Fatalf("same-control trigger provenance = %#v", u)
	}
	if p := outputFor(t, multipleTriggers, profile.KEY_P); len(p.Candidates) != 1 || p.Candidates[0].BindingIndex != 2 || p.Candidates[0].Trigger != profile.TriggerDouble {
		t.Fatalf("double trigger provenance = %#v", p)
	}

	compound := matchingControl(matchingID(4), profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Actions: []profile.Action{
		{Kind: profile.ActionKeyboard, Code: profile.KEY_U, Modifiers: []profile.CanonicalCode{profile.KEY_LEFTCTRL}},
		{Kind: profile.ActionKeyboard, Code: profile.KEY_U, Modifiers: []profile.CanonicalCode{profile.KEY_LEFTCTRL}},
	}})
	withModifier := Build(profile.Profile{Controls: []profile.ControlBinding{compound}}, matchingSource, definition)
	if len(withModifier.Outputs) != 2 || len(outputFor(t, withModifier, profile.KEY_U).Candidates) != 1 || len(outputFor(t, withModifier, profile.KEY_LEFTCTRL).Candidates) != 1 {
		t.Fatalf("modifier or within-binding duplicate lost: %#v", withModifier.Outputs)
	}

	functional := profile.Profile{Controls: []profile.ControlBinding{
		matchingControl(matchingID(4), profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingTurbo, Turbo: &profile.TurboBinding{Code: profile.KEY_T}}),
		matchingControl(matchingID(8), profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingMacro, Macro: &profile.MacroBinding{Steps: []profile.MacroStep{{Kind: profile.MacroStepButton, Code: profile.KEY_W}, {Kind: profile.MacroStepDelay}, {Kind: profile.MacroStepButton, Code: profile.KEY_W}}}}),
		matchingControl(matchingID(12), profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingStick, Stick: &profile.StickBinding{Mode: profile.StickModeKeyboard, KeyboardDirections: profile.KeyboardDirections{Up: profile.KEY_W, Right: profile.KEY_D, Down: profile.KEY_S, Left: profile.KEY_A}}}),
		matchingControl(matchingID(17), profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingStick, Stick: &profile.StickBinding{Mode: profile.StickModeXbox}}),
	}}
	allOutputs := Build(functional, matchingSource, definition)
	w := outputFor(t, allOutputs, profile.KEY_W)
	if len(w.Candidates) != 2 || !w.Ambiguous || w.Candidates[0].Kind != profile.BindingMacro || w.Candidates[1].Kind != profile.BindingStick || len(outputFor(t, allOutputs, profile.KEY_T).Candidates) != 1 {
		t.Fatalf("turbo/macro/stick candidates = %#v", allOutputs.Outputs)
	}
	var ordered []profile.CanonicalCode
	for _, output := range allOutputs.Outputs {
		ordered = append(ordered, output.Code)
	}
	if !reflect.DeepEqual(ordered, []profile.CanonicalCode{profile.KEY_A, profile.KEY_D, profile.KEY_S, profile.KEY_T, profile.KEY_W}) {
		t.Fatalf("output order = %#v", allOutputs.Outputs)
	}
}

func TestBuildMapping(t *testing.T) {
	t.Parallel()
	definition := matchingLayout(t)
	valid := matchingControl(matchingID(4), matchingKeyboard(profile.TriggerSingle, profile.KEY_U))
	cases := []struct {
		name   string
		edit   func(*profile.ControlBinding, *layout.Definition, *profile.SourceMetadata)
		mapped bool
		scope  bool
	}{
		{"export id only", func(*profile.ControlBinding, *layout.Definition, *profile.SourceMetadata) {}, true, true},
		{"local provenance", func(_ *profile.ControlBinding, _ *layout.Definition, s *profile.SourceMetadata) {
			s.SourceScope = "azeron-software-local-json"
		}, true, true},
		{"literal pin 255", func(c *profile.ControlBinding, _ *layout.Definition, _ *profile.SourceMetadata) {
			c.SourceIdentity.PinTwo = matchingID(255)
		}, true, true},
		{"missing id", func(c *profile.ControlBinding, _ *layout.Definition, _ *profile.SourceMetadata) {
			c.SourceIdentity.InputID = nil
			c.SourceIdentity.PinOne = matchingID(5)
		}, false, true},
		{"outside evidence", func(c *profile.ControlBinding, _ *layout.Definition, _ *profile.SourceMetadata) {
			c.SourceIdentity.InputID = matchingID(999)
		}, false, true},
		{"invalid identity", func(c *profile.ControlBinding, _ *layout.Definition, _ *profile.SourceMetadata) {
			c.SourceIdentity.Invalid = true
		}, false, true},
		{"pin conflict", func(c *profile.ControlBinding, _ *layout.Definition, _ *profile.SourceMetadata) {
			c.SourceIdentity.PinOne = matchingID(6)
		}, false, true},
		{"pin two conflict", func(c *profile.ControlBinding, _ *layout.Definition, _ *profile.SourceMetadata) {
			c.SourceIdentity.PinTwo = matchingID(0)
		}, false, true},
		{"source release", func(_ *profile.ControlBinding, _ *layout.Definition, s *profile.SourceMetadata) {
			s.SoftwareRelease = "2.0.3"
		}, false, false},
		{"source scope", func(_ *profile.ControlBinding, _ *layout.Definition, s *profile.SourceMetadata) {
			s.SourceScope = "unknown"
		}, false, false},
		{"layout hand", func(_ *profile.ControlBinding, d *layout.Definition, _ *profile.SourceMetadata) {
			d.Hand = "right"
		}, false, false},
		{"layout model", func(_ *profile.ControlBinding, d *layout.Definition, _ *profile.SourceMetadata) {
			d.Model = "other"
		}, false, false},
		{"evidence metadata is not caller context", func(_ *profile.ControlBinding, d *layout.Definition, _ *profile.SourceMetadata) {
			value := "rev-a"
			d.Applicability = layout.Applicability{SoftwareRelease: "other", DisplayedFirmware: "other", HardwareRevision: &value, Mode: "other"}
		}, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			control, def, source := valid, definition, matchingSource
			tc.edit(&control, &def, &source)
			index := Build(profile.Profile{Controls: []profile.ControlBinding{control}}, source, def)
			if index.ScopeSupported != tc.scope || len(index.Controls) != 1 || len(index.Outputs) != 1 || len(index.Outputs[0].Candidates) != 1 {
				t.Fatalf("scope/candidate coverage = %#v", index)
			}
			mapped := index.Controls[0].MappingState == MappingMapped
			if mapped != tc.mapped || (mapped && index.Controls[0].RegionID != "grid.c1.r1") || (!mapped && (index.Controls[0].RegionID != "" || !index.Outputs[0].HasUnresolvedCandidates)) {
				t.Fatalf("mapping = %#v", index)
			}
		})
	}
}

func TestBuildUnknownCoverage(t *testing.T) {
	t.Parallel()
	definition := matchingLayout(t)
	selected := profile.Profile{Controls: []profile.ControlBinding{
		matchingControl(matchingID(4), profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingUnbound}),
		matchingControl(matchingID(8), profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingUnknown, Unknown: &profile.UnknownBinding{Reason: "unmapped_binding"}}),
		matchingControl(matchingID(999), matchingKeyboard(profile.TriggerSingle, profile.KEY_U)),
	}}
	index := Build(selected, matchingSource, definition)
	if !index.HasUnknownBindings || len(index.Controls) != 3 || len(index.Outputs) != 1 || index.Controls[0].Bindings[0].Kind != profile.BindingUnbound || index.Controls[1].Bindings[0].Kind != profile.BindingUnknown {
		t.Fatalf("unknown and unbound coverage = %#v", index)
	}
	u := outputFor(t, index, profile.KEY_U)
	if len(u.Candidates) != 1 || u.Ambiguous || !u.HasUnresolvedCandidates || u.Candidates[0].RegionID != "" {
		t.Fatalf("unmapped known output disappeared or acquired a region: %#v", u)
	}
	selected.Controls = append(selected.Controls, matchingControl(matchingID(4), matchingKeyboard(profile.TriggerSingle, profile.KEY_U)))
	mixed := outputFor(t, Build(selected, matchingSource, definition), profile.KEY_U)
	if len(mixed.Candidates) != 2 || !mixed.Ambiguous || !mixed.HasUnresolvedCandidates || mixed.Candidates[0].MappingState != MappingUnresolved || mixed.Candidates[1].MappingState != MappingMapped {
		t.Fatalf("mapped and unresolved candidates were collapsed: %#v", mixed)
	}
}
