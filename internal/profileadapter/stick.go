package profileadapter

import (
	"encoding/json"
	"slices"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

func normalizeStick(raw profileraw.RawInput, bindings []profile.TriggerBinding) []profile.TriggerBinding {
	if angle, ok := matchesObservedXbox(raw); ok {
		bindings[0] = profile.TriggerBinding{
			Trigger: profile.TriggerSingle,
			Kind:    profile.BindingStick,
			Stick:   &profile.StickBinding{Mode: profile.StickModeXbox, AngleDegrees: angle},
		}
		return bindings
	}

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

func matchesObservedXbox(raw profileraw.RawInput) (int, bool) {
	if !matchesStringArray(raw, "types", []string{"21", "11", "11"}) ||
		raw["subType"] != nil || raw["isToggleOnHold"] != nil ||
		!matchesStringArray(raw, "keyValues", []string{"87", "0", "0", "0"}) ||
		!matchesStringArray(raw, "metaValues", []string{"0", "0", "3"}) ||
		!matchesFalse(raw, "isHold") || !matchesFalse(raw, "isTurbo") ||
		!matchesNumberToken(raw, "turboInterval", "0") || !matchesInactiveSlots(raw) {
		return 0, false
	}
	var settings profileraw.RawInput
	if json.Unmarshal(raw["analogSettings"], &settings) != nil {
		return 0, false
	}
	for _, field := range []string{"lowerLimit", "upperLimit", "sensitivity", "analogThrottle", "rotateStickButtonId"} {
		if !matchesNumberToken(settings, field, "0") {
			return 0, false
		}
	}
	for field, token := range map[string]string{
		"mouseSensitivity": "5", "triggerMagnitude": "4", "combinedAnalogMagnitude": "6",
		"holdMagnitude": "9", "lockZoneAngle": "70", "lockZoneSize": "30",
	} {
		if !matchesNumberToken(settings, field, token) {
			return 0, false
		}
	}
	if !matchesJSONString(settings["holdType"], "1") {
		return 0, false
	}
	for _, field := range []string{"isRightAnalog", "invertXAxis", "invertYAxis", "isCombinedAnalog", "isEightDirectionalTrigger", "isHoldTrigger", "isAnalogSmoothing", "isAngleLock"} {
		if !matchesFalse(settings, field) {
			return 0, false
		}
	}
	switch {
	case matchesNumberToken(settings, "angle", "0"):
		return 0, true
	case matchesNumberToken(settings, "angle", "90"):
		return 90, true
	default:
		return 0, false
	}
}
