package gameprofile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
)

type goldenRow struct {
	InputID        int                 `json:"input_id"`
	Trigger        profile.TriggerKind `json:"trigger"`
	Label          string              `json:"label"`
	BindingDisplay string              `json:"binding_display"`
}

func TestGolden(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content, err := os.ReadFile("testdata/sample.toml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "game.toml"), content, 0600); err != nil {
		t.Fatal(err)
	}
	catalog, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	game, ok := catalog.Lookup("sample")
	if !ok {
		t.Fatal("sample game missing")
	}
	source := "Azeron label"
	empty := ""
	keyboard := func(trigger profile.TriggerKind, codes ...profile.CanonicalCode) profile.TriggerBinding {
		actions := make([]profile.Action, 0, len(codes))
		for _, code := range codes {
			actions = append(actions, profile.Action{Kind: profile.ActionKeyboard, Code: code})
		}
		return profile.TriggerBinding{Trigger: trigger, Kind: profile.BindingKeyboard, Actions: actions}
	}
	controls := []profile.ControlBinding{
		{Label: &source, SourceIdentity: identity(15), Bindings: []profile.TriggerBinding{keyboard(profile.TriggerSingle, profile.KEY_U), keyboard(profile.TriggerLong, profile.KEY_U), keyboard(profile.TriggerDouble, profile.KEY_P, profile.KEY_L)}},
		{Label: &empty, SourceIdentity: identity(16), Bindings: []profile.TriggerBinding{keyboard(profile.TriggerSingle, profile.KEY_U)}},
		{SourceIdentity: identity(17), Bindings: []profile.TriggerBinding{keyboard(profile.TriggerSingle, profile.KEY_P)}},
		{SourceIdentity: identity(18), Bindings: []profile.TriggerBinding{{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Actions: []profile.Action{{Kind: profile.ActionKeyboard, Code: profile.KEY_U, Modifiers: []profile.CanonicalCode{profile.KEY_LEFTCTRL}}}}}},
		{SourceIdentity: identity(19), Bindings: []profile.TriggerBinding{keyboard(profile.TriggerDouble, profile.KEY_P, profile.KEY_L)}},
		{SourceIdentity: identity(20), Bindings: []profile.TriggerBinding{{Trigger: profile.TriggerSingle, Kind: profile.BindingUnknown, Unknown: &profile.UnknownBinding{Reason: "unmapped_binding", RawDisplay: `types[0]="3"`}}}},
		{SourceIdentity: identity(21), Bindings: []profile.TriggerBinding{{Trigger: profile.TriggerSingle, Kind: profile.BindingTurbo, Turbo: &profile.TurboBinding{Code: profile.KEY_U, ClicksPerSecond: 25}}}},
		{SourceIdentity: identity(22), Bindings: []profile.TriggerBinding{{Trigger: profile.TriggerSingle, Kind: profile.BindingStick, Stick: &profile.StickBinding{Mode: profile.StickModeXbox, AngleDegrees: 90}}}},
		{SourceIdentity: identity(23), Bindings: []profile.TriggerBinding{{Trigger: profile.TriggerSingle, Kind: profile.BindingUnbound}}},
	}
	var got []goldenRow
	for _, control := range controls {
		for _, binding := range control.Bindings {
			resolved := Resolve(control, binding, game)
			got = append(got, goldenRow{InputID: *control.SourceIdentity.InputID, Trigger: binding.Trigger, Label: resolved.Label, BindingDisplay: resolved.BindingDisplay})
		}
	}
	data, err := os.ReadFile("testdata/sample.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var want []goldenRow
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		actual, _ := json.MarshalIndent(got, "", "  ")
		t.Fatalf("resolved rows:\n%s\nwant:\n%s", actual, data)
	}
}

func identity(id int) profile.SourceIdentity { return profile.SourceIdentity{InputID: &id} }
