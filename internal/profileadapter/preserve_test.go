package profileadapter_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

var admittedSource = profile.SourceMetadata{
	SoftwareRelease: "2.0.2",
	SourceScope:     "azeron-software-export",
}

func TestNormalizePreservation(t *testing.T) {
	t.Parallel()

	raw := parseExport(t, `{"version":1e+09,"profiles":[{"id":"same","name":null,"version":-0,"inputs":[{"label":"first","types":["1","11","11"],"future":{"n":1e+09}}],"profileBool":true},{"id":"same","name":"second","inputs":[],"profileArray":[]}],"bundleNull":null}`)

	actual, err := profileadapter.Normalize(raw, admittedSource)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	want := profile.ProfileBundle{
		SchemaVersion: 1,
		Source:        admittedSource,
		RootKind:      profile.RootBundle,
		Profiles: []profile.Profile{
			{
				ID: stringPointer("same"),
				Controls: []profile.ControlBinding{{
					Label: stringPointer("first"),
					Bindings: []profile.TriggerBinding{
						{Trigger: profile.TriggerSingle, Kind: profile.BindingUnknown, Unknown: &profile.UnknownBinding{Reason: "unmapped_binding", RawDisplay: `types[0]="1"`}},
						{Trigger: profile.TriggerLong, Kind: profile.BindingUnknown, Unknown: &profile.UnknownBinding{Reason: "unmapped_binding", RawDisplay: `types[1]="11"`}},
						{Trigger: profile.TriggerDouble, Kind: profile.BindingUnknown, Unknown: &profile.UnknownBinding{Reason: "unmapped_binding", RawDisplay: `types[2]="11"`}},
					},
					Raw: profile.RawBindingReference{
						RootKind:     profile.RootBundle,
						ProfileIndex: 0,
						InputIndex:   0,
						Fields:       rawMap(map[string]string{"label": `"first"`, "types": `["1","11","11"]`, "future": `{"n":1e+09}`}),
					},
				}},
				Raw: profile.RawProfileReference{
					ID:      json.RawMessage(`"same"`),
					Name:    json.RawMessage(`null`),
					Version: json.RawMessage(`-0`),
					Unknown: rawMap(map[string]string{"profileBool": `true`}),
				},
			},
			{
				ID:       stringPointer("same"),
				Name:     stringPointer("second"),
				Controls: []profile.ControlBinding{},
				Raw: profile.RawProfileReference{
					ID:      json.RawMessage(`"same"`),
					Name:    json.RawMessage(`"second"`),
					Unknown: rawMap(map[string]string{"profileArray": `[]`}),
				},
			},
		},
		Raw: &profile.RawBundleReference{
			Version: json.RawMessage(`1e+09`),
			Unknown: rawMap(map[string]string{"bundleNull": `null`}),
		},
	}
	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("Normalize() = %#v, want %#v", actual, want)
	}

	mutateRawExport(raw)
	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("Normalize() result changed after caller mutation: %#v", actual)
	}

	fresh := parseExport(t, `{"id":"owned","name":"name","version":3e2,"inputs":[{"label":"label","types":["11","11","11"],"opaque":[1]}],"extra":{"ok":true}}`)
	first, err := profileadapter.Normalize(fresh, admittedSource)
	if err != nil {
		t.Fatalf("first Normalize() error = %v", err)
	}
	second, err := profileadapter.Normalize(fresh, admittedSource)
	if err != nil {
		t.Fatalf("second Normalize() error = %v", err)
	}
	mutateNormalized(&first)
	if second.Profiles[0].ID == nil || *second.Profiles[0].ID != "owned" || second.Profiles[0].Name == nil || *second.Profiles[0].Name != "name" || second.Profiles[0].Controls[0].Label == nil || *second.Profiles[0].Controls[0].Label != "label" {
		t.Fatalf("second Normalize() text values aliased first: %#v", second)
	}
	if string(second.Profiles[0].Raw.ID) != `"owned"` || string(second.Profiles[0].Controls[0].Raw.Fields["opaque"]) != `[1]` || second.Profiles[0].Controls[0].Bindings[0].Unknown.Reason != "unmapped_binding" {
		t.Fatalf("second Normalize() raw/outcome values aliased first: %#v", second)
	}
}

func TestNormalizeLabels(t *testing.T) {
	t.Parallel()

	raw := parseExport(t, `{"id":"labels","inputs":[{"types":null},{"label":"","types":null},{"label":"shown","types":null},{"label":null,"types":null},{"label":7,"types":null},{"label":false,"types":null},{"label":{},"types":null},{"label":[],"types":null}]}`)

	actual, err := profileadapter.Normalize(raw, admittedSource)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	labels := []*string{nil, stringPointer(""), stringPointer("shown"), nil, nil, nil, nil, nil}
	rawLabels := []string{"", `""`, `"shown"`, `null`, `7`, `false`, `{}`, `[]`}
	for index, control := range actual.Profiles[0].Controls {
		if !reflect.DeepEqual(control.Label, labels[index]) {
			t.Errorf("control %d label = %#v, want %#v", index, control.Label, labels[index])
		}
		if string(control.Raw.Fields["label"]) != rawLabels[index] {
			t.Errorf("control %d raw label = %q, want %q", index, control.Raw.Fields["label"], rawLabels[index])
		}
	}
}

func TestNormalizeUnknown(t *testing.T) {
	t.Parallel()

	raw := parseExport(t, `{"name":"unknown","inputs":[{"types":["1","1","1"],"keyValues":[99],"setting":null},{"types":["11","11","11"]},{"types":["16","11","11"]},{"types":[1,11,11]},{"types":["1"]},{"value":42}]}`)

	actual, err := profileadapter.Normalize(raw, admittedSource)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	wantTriggers := [][]profile.TriggerKind{
		{profile.TriggerSingle, profile.TriggerLong, profile.TriggerDouble},
		{profile.TriggerSingle, profile.TriggerLong, profile.TriggerDouble},
		{profile.TriggerSingle, profile.TriggerLong, profile.TriggerDouble},
		{profile.TriggerUnknown}, {profile.TriggerUnknown}, {profile.TriggerUnknown},
	}
	for controlIndex, control := range actual.Profiles[0].Controls {
		if len(control.Bindings) != len(wantTriggers[controlIndex]) {
			t.Fatalf("control %d binding count = %d, want %d", controlIndex, len(control.Bindings), len(wantTriggers[controlIndex]))
		}
		for bindingIndex, binding := range control.Bindings {
			if binding.Trigger != wantTriggers[controlIndex][bindingIndex] || binding.Kind != profile.BindingUnknown || binding.Unknown == nil || binding.Unknown.Reason != "unmapped_binding" {
				t.Errorf("control %d binding %d = %#v", controlIndex, bindingIndex, binding)
			}
			if binding.Actions != nil || binding.TriggerDelayMS != nil || binding.TriggerIntervalMS != nil || binding.ReleaseBehavior != nil {
				t.Errorf("control %d binding %d gained semantics: %#v", controlIndex, bindingIndex, binding)
			}
		}
		if len(control.Raw.Fields) != len(raw.Single.Inputs[controlIndex]) {
			t.Errorf("control %d retained fields = %d, want %d", controlIndex, len(control.Raw.Fields), len(raw.Single.Inputs[controlIndex]))
		}
	}
}

func TestNormalizeOrdering(t *testing.T) {
	t.Parallel()

	raw := parseExport(t, `{"profiles":[{"id":"duplicate","inputs":[{"label":"a","types":["1","11","11"]},{"label":"a","types":["11","1","11"]}]},{"id":"duplicate","inputs":[{"label":"a","types":["11","11","1"]}]}]}`)

	actual, err := profileadapter.Normalize(raw, admittedSource)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if len(actual.Profiles) != 2 || len(actual.Profiles[0].Controls) != 2 || len(actual.Profiles[1].Controls) != 1 {
		t.Fatalf("ordered duplicates were coalesced: %#v", actual.Profiles)
	}
	for profileIndex, normalizedProfile := range actual.Profiles {
		if normalizedProfile.ID == nil || *normalizedProfile.ID != "duplicate" {
			t.Errorf("profile %d ID = %#v", profileIndex, normalizedProfile.ID)
		}
		for inputIndex, control := range normalizedProfile.Controls {
			if control.Raw.RootKind != profile.RootBundle || control.Raw.ProfileIndex != profileIndex || control.Raw.InputIndex != inputIndex {
				t.Errorf("control provenance = %#v, want bundle/%d/%d", control.Raw, profileIndex, inputIndex)
			}
			if got := []profile.TriggerKind{control.Bindings[0].Trigger, control.Bindings[1].Trigger, control.Bindings[2].Trigger}; !reflect.DeepEqual(got, []profile.TriggerKind{profile.TriggerSingle, profile.TriggerLong, profile.TriggerDouble}) {
				t.Errorf("control triggers = %v", got)
			}
		}
	}

	single := parseExport(t, `{"name":"single","inputs":[{"types":null},{"types":null}]}`)
	singleResult, err := profileadapter.Normalize(single, admittedSource)
	if err != nil {
		t.Fatalf("Normalize(single) error = %v", err)
	}
	if singleResult.RootKind != profile.RootSingle || singleResult.Raw != nil || len(singleResult.Profiles) != 1 || singleResult.Profiles[0].Controls[1].Raw.ProfileIndex != 0 || singleResult.Profiles[0].Controls[1].Raw.InputIndex != 1 {
		t.Fatalf("single root identity/order = %#v", singleResult)
	}
}

func rawMap(values map[string]string) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage, len(values))
	for key, value := range values {
		result[key] = json.RawMessage(value)
	}
	return result
}

func stringPointer(value string) *string { return &value }

func mutateRawExport(raw profileraw.RawExport) {
	raw.Bundle.Version.Raw[0] = '9'
	raw.Bundle.Unknown["bundleNull"][0] = '1'
	raw.Bundle.Unknown["added"] = json.RawMessage(`true`)
	raw.Bundle.Profiles[0].ID.Raw[1] = 'X'
	raw.Bundle.Profiles[0].Name.Raw[0] = '0'
	raw.Bundle.Profiles[0].Version.Raw[0] = '8'
	raw.Bundle.Profiles[0].Unknown["profileBool"][0] = 'f'
	raw.Bundle.Profiles[0].Unknown["added"] = json.RawMessage(`true`)
	raw.Bundle.Profiles[0].Inputs[0]["label"][1] = 'X'
	raw.Bundle.Profiles[0].Inputs[0]["added"] = json.RawMessage(`true`)
	raw.Bundle.Profiles[0].Inputs = append(raw.Bundle.Profiles[0].Inputs, profileraw.RawInput{})
	raw.Bundle.Profiles = append(raw.Bundle.Profiles, profileraw.RawProfile{})
}

func mutateNormalized(bundle *profile.ProfileBundle) {
	*bundle.Profiles[0].ID = "changed"
	*bundle.Profiles[0].Name = "changed"
	*bundle.Profiles[0].Controls[0].Label = "changed"
	bundle.Profiles[0].Raw.ID[1] = 'X'
	bundle.Profiles[0].Raw.Unknown["extra"][0] = '0'
	bundle.Profiles[0].Controls[0].Raw.Fields["opaque"][1] = '9'
	bundle.Profiles[0].Controls[0].Bindings[0].Unknown.Reason = "changed"
	bundle.Profiles[0].Controls = append(bundle.Profiles[0].Controls, profile.ControlBinding{})
	bundle.Profiles = append(bundle.Profiles, profile.Profile{})
}
