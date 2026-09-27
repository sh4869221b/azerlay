package gameprofile

import (
	"encoding/json"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
)

func TestResolve(t *testing.T) {
	t.Parallel()
	id15, id16, zero := 15, 16, 0
	empty, source, space := "", "Azeron label", "  "
	keyboard := profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Actions: []profile.Action{{Kind: profile.ActionKeyboard, Code: profile.KEY_U}}}
	long := keyboard
	long.Trigger = profile.TriggerLong
	double := keyboard
	double.Trigger = profile.TriggerDouble
	chord := profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Actions: []profile.Action{{Kind: profile.ActionKeyboard, Code: profile.KEY_U, Modifiers: []profile.CanonicalCode{profile.KEY_LEFTCTRL}}}}
	multiple := profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Actions: []profile.Action{{Kind: profile.ActionKeyboard, Code: profile.KEY_P}, {Kind: profile.ActionKeyboard, Code: profile.KEY_L}}}
	turbo := profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingTurbo, Turbo: &profile.TurboBinding{Code: profile.KEY_U, ClicksPerSecond: 25}}
	macro := profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingMacro, Macro: &profile.MacroBinding{Steps: []profile.MacroStep{{Kind: profile.MacroStepButton, Code: profile.KEY_U}}}}
	unknown := profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingUnknown, Unknown: &profile.UnknownBinding{Reason: "unmapped_binding", RawDisplay: `types[0]="3"`}}
	game := Definition{
		Bindings: map[profile.CanonicalCode]string{profile.KEY_U: "Game U", profile.KEY_P: "Game P"},
		Controls: map[ControlKey]string{{InputID: 15, Trigger: profile.TriggerSingle}: "Physical 15"},
	}
	tests := []struct {
		name    string
		control profile.ControlBinding
		binding profile.TriggerBinding
		game    Definition
		label   string
		display string
	}{
		{"physical beats source and game", profile.ControlBinding{Label: &source, SourceIdentity: profile.SourceIdentity{InputID: &id15}}, keyboard, game, "Physical 15", "U"},
		{"source beats game", profile.ControlBinding{Label: &source, SourceIdentity: profile.SourceIdentity{InputID: &id16}}, keyboard, game, source, "U"},
		{"source whitespace is literal", profile.ControlBinding{Label: &space}, keyboard, game, space, "U"},
		{"empty source uses game", profile.ControlBinding{Label: &empty}, keyboard, game, "Game U", "U"},
		{"nil source uses game", profile.ControlBinding{}, keyboard, game, "Game U", "U"},
		{"missing game uses canonical", profile.ControlBinding{}, keyboard, Definition{}, "U", "U"},
		{"other control sharing output", profile.ControlBinding{SourceIdentity: profile.SourceIdentity{InputID: &id16}}, keyboard, game, "Game U", "U"},
		{"other trigger on same control", profile.ControlBinding{SourceIdentity: profile.SourceIdentity{InputID: &id15}}, long, game, "Game U", "U"},
		{"double trigger on same control", profile.ControlBinding{SourceIdentity: profile.SourceIdentity{InputID: &id15}}, double, game, "Game U", "U"},
		{"invalid identity", profile.ControlBinding{SourceIdentity: profile.SourceIdentity{InputID: &id15, Invalid: true}}, keyboard, game, "Game U", "U"},
		{"zero identity", profile.ControlBinding{SourceIdentity: profile.SourceIdentity{InputID: &zero}}, keyboard, game, "Game U", "U"},
		{"modified key is distinct", profile.ControlBinding{}, chord, game, "Left Ctrl+U", "Left Ctrl+U"},
		{"multiple actions are distinct", profile.ControlBinding{}, multiple, game, "P, L", "P, L"},
		{"turbo code uses game", profile.ControlBinding{}, turbo, game, "Game U", "Turbo (U, 25/s)"},
		{"macro step cannot use game", profile.ControlBinding{}, macro, game, "Macro", "Macro"},
		{"unknown summary ignores raw map", profile.ControlBinding{Raw: profile.RawBindingReference{Fields: map[string]json.RawMessage{"private": json.RawMessage(`"PRIVATE_LABEL"`)}}}, unknown, game, `Unknown (types[0]="3")`, `Unknown (types[0]="3")`},
		{"unknown physical override", profile.ControlBinding{SourceIdentity: profile.SourceIdentity{InputID: &id15}}, unknown, game, "Physical 15", `Unknown (types[0]="3")`},
		{"unbound canonical", profile.ControlBinding{}, profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingUnbound}, game, "Unbound", "Unbound"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Resolve(test.control, test.binding, test.game)
			if got.Label != test.label || got.BindingDisplay != test.display {
				t.Fatalf("Resolve() = %+v; want label %q, display %q", got, test.label, test.display)
			}
		})
	}

	t.Run("whitespace game labels fall through", func(t *testing.T) {
		blank := Definition{Bindings: map[profile.CanonicalCode]string{profile.KEY_U: " \t"}, Controls: map[ControlKey]string{{InputID: 15, Trigger: profile.TriggerSingle}: " \t"}}
		got := Resolve(profile.ControlBinding{SourceIdentity: profile.SourceIdentity{InputID: &id15}}, keyboard, blank)
		if got != (Resolved{Label: "U", BindingDisplay: "U"}) {
			t.Fatalf("blank game labels = %+v", got)
		}
	})
}
