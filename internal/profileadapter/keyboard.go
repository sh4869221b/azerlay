package profileadapter

import (
	"bytes"
	"encoding/json"
	"slices"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

var (
	allTriggerTypes    = []string{"1", "1", "1"}
	singleTriggerTypes = []string{"1", "11", "11"}
	longTriggerTypes   = []string{"11", "1", "11"}
	doubleTriggerTypes = []string{"11", "11", "1"}
	zeroKeys           = []string{"0", "0", "0", "0"}
	zeroModifiers      = []string{"0", "0", "0"}
)

func normalizeKeyboard(raw profileraw.RawInput, bindings []profile.TriggerBinding) []profile.TriggerBinding {
	switch {
	case matchesStringArray(raw, "types", allTriggerTypes):
		if matchesOriginalLong(raw) {
			delay := 500
			release := "regular"
			bindings[1] = profile.TriggerBinding{
				Trigger:         profile.TriggerLong,
				Kind:            profile.BindingKeyboard,
				Actions:         []profile.Action{{Kind: profile.ActionKeyboard, Code: profile.KEY_U}},
				TriggerDelayMS:  &delay,
				ReleaseBehavior: &release,
			}
		}
		if matchesOriginalDouble(raw) {
			interval := 150
			bindings[2] = profile.TriggerBinding{
				Trigger: profile.TriggerDouble,
				Kind:    profile.BindingKeyboard,
				Actions: []profile.Action{
					{Kind: profile.ActionKeyboard, Code: profile.KEY_P},
					{Kind: profile.ActionKeyboard, Code: profile.KEY_L},
				},
				TriggerIntervalMS: &interval,
			}
		}
	case matchesStringArray(raw, "types", singleTriggerTypes):
		bindings[0] = normalizeFollowupSingle(raw, bindings[0])
	case matchesStringArray(raw, "types", longTriggerTypes):
		if matchesFollowupLong(raw) {
			delay := 1278
			release := "regular"
			bindings[1] = profile.TriggerBinding{
				Trigger:         profile.TriggerLong,
				Kind:            profile.BindingKeyboard,
				Actions:         []profile.Action{{Kind: profile.ActionKeyboard, Code: profile.KEY_U}},
				TriggerDelayMS:  &delay,
				ReleaseBehavior: &release,
			}
		}
	case matchesStringArray(raw, "types", doubleTriggerTypes):
		if matchesFollowupDouble(raw) {
			interval := 123
			bindings[2] = profile.TriggerBinding{
				Trigger:           profile.TriggerDouble,
				Kind:              profile.BindingKeyboard,
				Actions:           []profile.Action{{Kind: profile.ActionKeyboard, Code: profile.KEY_I}},
				TriggerIntervalMS: &interval,
			}
		}
	}
	return bindings
}

func matchesOriginalLong(raw profileraw.RawInput) bool {
	return matchesStringArray(raw, "keyValuesLong", []string{"KeyU", "0", "0", "0"}) &&
		matchesStringArray(raw, "metaValuesLong", zeroModifiers) &&
		matchesFalse(raw, "isHoldLong") && matchesFalse(raw, "isTurboLong") && matchesFalse(raw, "isToggleOnHoldLong") &&
		matchesNumberToken(raw, "featureDelay", "500")
}

func matchesOriginalDouble(raw profileraw.RawInput) bool {
	return matchesStringArray(raw, "keyValuesDouble", []string{"KeyP", "KeyL", "0", "0"}) &&
		matchesStringArray(raw, "metaValuesDouble", zeroModifiers) && matchesNumberToken(raw, "doubleDelay", "150")
}

func normalizeFollowupSingle(raw profileraw.RawInput, unknown profile.TriggerBinding) profile.TriggerBinding {
	if !matchesFalse(raw, "isHold") || !matchesFalse(raw, "isTurbo") || !matchesFalse(raw, "isToggleOnHold") ||
		!matchesStringArray(raw, "keyValuesLong", zeroKeys) || !matchesStringArray(raw, "metaValuesLong", zeroModifiers) ||
		!matchesStringArray(raw, "keyValuesDouble", zeroKeys) || !matchesStringArray(raw, "metaValuesDouble", zeroModifiers) {
		return unknown
	}
	release := "regular"
	switch {
	case matchesStringArray(raw, "keyValues", []string{"KeyU", "0", "0", "0"}) && matchesStringArray(raw, "metaValues", zeroModifiers):
		return profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Actions: []profile.Action{{Kind: profile.ActionKeyboard, Code: profile.KEY_U}}, ReleaseBehavior: &release}
	case matchesStringArray(raw, "keyValues", []string{"KeyP", "0", "0", "0"}) && matchesStringArray(raw, "metaValues", zeroModifiers):
		return profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Actions: []profile.Action{{Kind: profile.ActionKeyboard, Code: profile.KEY_P}}, ReleaseBehavior: &release}
	case matchesStringArray(raw, "keyValues", []string{"KeyU", "0", "0", "0"}) && matchesStringArray(raw, "metaValues", []string{"ControlLeft", "0", "0"}):
		return profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Actions: []profile.Action{{Kind: profile.ActionKeyboard, Code: profile.KEY_U, Modifiers: []profile.CanonicalCode{profile.KEY_LEFTCTRL}}}, ReleaseBehavior: &release}
	default:
		return unknown
	}
}

func matchesFollowupLong(raw profileraw.RawInput) bool {
	return matchesStringArray(raw, "keyValues", zeroKeys) && matchesStringArray(raw, "metaValues", zeroModifiers) &&
		matchesStringArray(raw, "keyValuesLong", []string{"KeyU", "0", "0", "0"}) && matchesStringArray(raw, "metaValuesLong", zeroModifiers) &&
		matchesFalse(raw, "isHoldLong") && matchesFalse(raw, "isTurboLong") && matchesFalse(raw, "isToggleOnHoldLong") &&
		matchesNumberToken(raw, "featureDelay", "1278") &&
		matchesStringArray(raw, "keyValuesDouble", zeroKeys) && matchesStringArray(raw, "metaValuesDouble", zeroModifiers)
}

func matchesFollowupDouble(raw profileraw.RawInput) bool {
	return matchesStringArray(raw, "keyValues", zeroKeys) && matchesStringArray(raw, "metaValues", zeroModifiers) &&
		matchesStringArray(raw, "keyValuesLong", zeroKeys) && matchesStringArray(raw, "metaValuesLong", zeroModifiers) &&
		matchesStringArray(raw, "keyValuesDouble", []string{"KeyI", "0", "0", "0"}) && matchesStringArray(raw, "metaValuesDouble", zeroModifiers) &&
		matchesFalse(raw, "isHoldDouble") && matchesFalse(raw, "isTurboDouble") && matchesFalse(raw, "isToggleOnHoldDouble") &&
		matchesNumberToken(raw, "doubleDelay", "123")
}

func matchesStringArray(raw profileraw.RawInput, field string, expected []string) bool {
	var actual []string
	return json.Unmarshal(raw[field], &actual) == nil && slices.Equal(actual, expected)
}

func matchesFalse(raw profileraw.RawInput, field string) bool {
	var value bool
	return json.Unmarshal(raw[field], &value) == nil && !value && bytes.Equal(bytes.TrimSpace(raw[field]), []byte("false"))
}

func matchesNumberToken(raw profileraw.RawInput, field, expected string) bool {
	return bytes.Equal(bytes.TrimSpace(raw[field]), []byte(expected))
}
