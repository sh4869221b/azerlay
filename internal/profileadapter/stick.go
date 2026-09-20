package profileadapter

import (
	"encoding/json"
	"slices"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

func normalizeStick(raw profileraw.RawInput, bindings []profile.TriggerBinding) []profile.TriggerBinding {
	var mode profile.StickMode
	switch {
	case matchesStringArray(raw, "types", []string{"4", "11", "11"}):
		mode = profile.StickModeKeyboard
	case matchesStringArray(raw, "types", []string{"21", "11", "11"}):
		mode = profile.StickModeXbox
	default:
		return bindings
	}
	var subType string
	if json.Unmarshal(raw["subType"], &subType) != nil || subType != "11" ||
		!matchesFalse(raw, "isHold") || !matchesFalse(raw, "isTurbo") || !matchesFalse(raw, "isToggleOnHold") ||
		!matchesStringArray(raw, "keyValues", zeroKeys) || !matchesStringArray(raw, "metaValues", zeroModifiers) {
		return bindings
	}
	var settings profileraw.RawInput
	if json.Unmarshal(raw["analogSettings"], &settings) != nil {
		return bindings
	}
	for _, field := range []string{"angle", "lowerLimit", "upperLimit"} {
		if !matchesNumberToken(settings, field, "0") {
			return bindings
		}
	}
	for _, field := range []string{"isRightAnalog", "invertXAxis", "invertYAxis", "isCombinedAnalog", "isEightDirectionalTrigger", "isHoldTrigger", "isAnalogSmoothing", "isAngleLock"} {
		if !matchesFalse(settings, field) {
			return bindings
		}
	}
	stick := &profile.StickBinding{Mode: mode}
	if mode == profile.StickModeKeyboard {
		var keys struct {
			Left map[string]json.RawMessage `json:"left"`
		}
		if json.Unmarshal(settings["analogKeys"], &keys) != nil {
			return bindings
		}
		for _, direction := range []struct {
			name string
			key  string
		}{{"up", "87"}, {"right", "68"}, {"down", "83"}, {"left", "65"}} {
			var tuple []json.RawMessage
			if json.Unmarshal(keys.Left[direction.name], &tuple) != nil ||
				!slices.EqualFunc(tuple, []json.RawMessage{json.RawMessage(direction.key), json.RawMessage("0"), json.RawMessage("0")}, func(a, b json.RawMessage) bool { return string(a) == string(b) }) {
				return bindings
			}
		}
		stick.KeyboardDirections = profile.KeyboardDirections{Up: profile.KEY_W, Right: profile.KEY_D, Down: profile.KEY_S, Left: profile.KEY_A}
	}
	bindings[0] = profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingStick, Stick: stick}
	return bindings
}
