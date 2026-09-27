package gameprofile

import (
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
)

func TestBindingDisplay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		binding profile.TriggerBinding
		want    string
	}{
		{"ordered keyboard actions", profile.TriggerBinding{Kind: profile.BindingKeyboard, Actions: []profile.Action{{Kind: profile.ActionKeyboard, Code: profile.KEY_U, Modifiers: []profile.CanonicalCode{"KEY_LEFTCTRL", "KEY_RIGHTSHIFT"}}, {Kind: profile.ActionKeyboard, Code: "BTN_SOUTH"}, {Kind: profile.ActionKeyboard, Code: "OTHER"}}}, "Left Ctrl+Right Shift+U, Button SOUTH, OTHER"},
		{"right modifiers", profile.TriggerBinding{Kind: profile.BindingKeyboard, Actions: []profile.Action{{Kind: profile.ActionKeyboard, Code: "KEY_RIGHTMETA", Modifiers: []profile.CanonicalCode{"KEY_RIGHTCTRL", "KEY_LEFTSHIFT", "KEY_LEFTALT", "KEY_RIGHTALT", "KEY_LEFTMETA"}}}}, "Right Ctrl+Left Shift+Left Alt+Right Alt+Left Meta+Right Meta"},
		{"turbo", profile.TriggerBinding{Kind: profile.BindingTurbo, Turbo: &profile.TurboBinding{Code: "BTN_SOUTH", ClicksPerSecond: 10}}, "Turbo (Button SOUTH, 10/s)"},
		{"macro", profile.TriggerBinding{Kind: profile.BindingMacro, Macro: &profile.MacroBinding{Steps: []profile.MacroStep{{Kind: profile.MacroStepButton, Code: profile.KEY_U}}}}, "Macro"},
		{"repeat macro", profile.TriggerBinding{Kind: profile.BindingMacro, Macro: &profile.MacroBinding{RepeatWhileHeld: true, Steps: []profile.MacroStep{{Kind: profile.MacroStepButton, Code: profile.KEY_U}}}}, "Macro (repeat while held)"},
		{"xbox stick", profile.TriggerBinding{Kind: profile.BindingStick, Stick: &profile.StickBinding{Mode: profile.StickModeXbox, AngleDegrees: 90}}, "Left Stick (Xbox, 90 deg)"},
		{"keyboard stick", profile.TriggerBinding{Kind: profile.BindingStick, Stick: &profile.StickBinding{Mode: profile.StickModeKeyboard, KeyboardDirections: profile.KeyboardDirections{Up: profile.KEY_W, Right: profile.KEY_D, Down: profile.KEY_S, Left: profile.KEY_A}}}, "Stick (Up: W, Right: D, Down: S, Left: A, 0 deg)"},
		{"unbound", profile.TriggerBinding{Kind: profile.BindingUnbound}, "Unbound"},
		{"unknown raw", profile.TriggerBinding{Kind: profile.BindingUnknown, Unknown: &profile.UnknownBinding{Reason: "unmapped_binding", RawDisplay: `types[0]="3"`}}, `Unknown (types[0]="3")`},
		{"legacy unknown", profile.TriggerBinding{Kind: profile.BindingUnknown, Unknown: &profile.UnknownBinding{Reason: "unmapped_binding"}}, "Unknown (unmapped_binding)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := bindingDisplay(test.binding)
			if got != test.want {
				t.Fatalf("bindingDisplay() = %q; want %q", got, test.want)
			}
		})
	}
}
