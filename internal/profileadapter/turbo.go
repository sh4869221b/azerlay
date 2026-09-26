package profileadapter

import (
	"bytes"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

func normalizeTurbo(raw profileraw.RawInput, bindings []profile.TriggerBinding) []profile.TriggerBinding {
	if !matchesStringArray(raw, "types", singleTriggerTypes) ||
		!matchesStringArray(raw, "keyValues", []string{"KeyT", "0", "0", "0"}) ||
		!matchesStringArray(raw, "metaValues", zeroModifiers) ||
		!matchesFalse(raw, "isHold") || !matchesTrue(raw, "isTurbo") ||
		raw["isToggleOnHold"] != nil || !matchesInactiveSlots(raw) {
		return bindings
	}

	var rate int
	switch {
	case matchesNumberToken(raw, "turboInterval", "20"):
		rate = 25
	case matchesNumberToken(raw, "turboInterval", "50"):
		rate = 10
	default:
		return bindings
	}
	bindings[0] = profile.TriggerBinding{
		Trigger: profile.TriggerSingle,
		Kind:    profile.BindingTurbo,
		Turbo:   &profile.TurboBinding{Code: profile.KEY_T, ClicksPerSecond: rate},
	}
	return bindings
}

func matchesTrue(raw profileraw.RawInput, field string) bool {
	return bytes.Equal(bytes.TrimSpace(raw[field]), []byte("true"))
}

func matchesInactiveSlots(raw profileraw.RawInput) bool {
	return matchesStringArray(raw, "keyValuesLong", zeroKeys) &&
		matchesStringArray(raw, "metaValuesLong", zeroModifiers) &&
		matchesStringArray(raw, "keyValuesDouble", zeroKeys) &&
		matchesStringArray(raw, "metaValuesDouble", zeroModifiers) &&
		matchesFalse(raw, "isHoldLong") && matchesFalse(raw, "isHoldDouble") &&
		matchesFalse(raw, "isTurboLong") && matchesFalse(raw, "isTurboDouble") &&
		matchesNumberToken(raw, "turboIntervalLong", "0") &&
		matchesNumberToken(raw, "turboIntervalDouble", "0")
}
