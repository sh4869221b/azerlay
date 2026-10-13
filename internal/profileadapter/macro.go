package profileadapter

import (
	"bytes"
	"encoding/json"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

const maxRecognizedMacroSteps = 1000

func normalizeMacro(raw profileraw.RawInput, bindings []profile.TriggerBinding) {
	if !matchesStringArray(raw, "types", []string{"16", "11", "11"}) ||
		!matchesStringArray(raw, "keyValues", zeroKeys) ||
		!matchesStringArray(raw, "metaValues", zeroModifiers) ||
		!matchesFalse(raw, "isHold") || !matchesFalse(raw, "isTurbo") ||
		!matchesNumberToken(raw, "turboInterval", "0") ||
		raw["isToggleOnHold"] != nil || !matchesInactiveSlots(raw) {
		return
	}

	var macro map[string]json.RawMessage
	if json.Unmarshal(raw["macro"], &macro) != nil || len(macro) != 3 ||
		!matchesNumberToken(macro, "v", "1") || !isJSONBoolean(macro["repeat"]) {
		return
	}
	steps := newRawArray(macro["steps"])
	buttonToken, buttonOK := steps.next()
	delayToken, delayOK := steps.next()
	if !buttonOK || !delayOK || !steps.ended() {
		return
	}
	var button, delay map[string]json.RawMessage
	if json.Unmarshal(buttonToken, &button) != nil || len(button) != 4 ||
		!matchesJSONString(button["type"], "Button") || !matchesJSONString(button["direction"], "Full") ||
		!matchesNumberToken(button, "duration", "50") || !matchesNumberToken(button, "keyCode", "87") ||
		json.Unmarshal(delayToken, &delay) != nil || len(delay) != 3 ||
		!matchesJSONString(delay["type"], "Delay") || !matchesJSONString(delay["direction"], "Full") ||
		!matchesNumberToken(delay, "duration", "100") {
		return
	}

	bindings[0] = profile.TriggerBinding{
		Trigger: profile.TriggerSingle,
		Kind:    profile.BindingMacro,
		Macro: &profile.MacroBinding{
			RepeatWhileHeld: bytes.Equal(bytes.TrimSpace(macro["repeat"]), []byte("true")),
			Steps: []profile.MacroStep{
				{Kind: profile.MacroStepButton, Code: profile.KEY_W, DurationMS: 50},
				{Kind: profile.MacroStepDelay, DurationMS: 100},
			},
		},
	}
}

func recognizedMacroExceedsLimit(raw profileraw.RawInput) bool {
	if !matchesStringArray(raw, "types", []string{"16", "11", "11"}) {
		return false
	}

	macroToken := bytes.TrimSpace(raw["macro"])
	if len(macroToken) == 0 || macroToken[0] != '{' {
		return false
	}
	var macro map[string]json.RawMessage
	if json.Unmarshal(macroToken, &macro) != nil || len(macro) != 3 {
		return false
	}
	if !bytes.Equal(bytes.TrimSpace(macro["v"]), []byte("1")) || !isJSONBoolean(macro["repeat"]) {
		return false
	}

	stepsToken := bytes.TrimSpace(macro["steps"])
	if len(stepsToken) == 0 || stepsToken[0] != '[' {
		return false
	}
	steps := newRawArray(stepsToken)
	count := 0
	for {
		step, ok := steps.next()
		if !ok {
			break
		}
		if !isRecognizedMacroStep(step) {
			return false
		}
		count++
	}
	return steps.ended() && count > maxRecognizedMacroSteps
}

func isRecognizedMacroStep(raw json.RawMessage) bool {
	token := bytes.TrimSpace(raw)
	if len(token) == 0 || token[0] != '{' {
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(token, &fields) != nil || !matchesJSONString(fields["direction"], "Full") || !isJSONNumber(fields["duration"]) {
		return false
	}

	switch {
	case matchesJSONString(fields["type"], "Button"):
		return len(fields) == 4 && (isJSONNumber(fields["keyCode"]) || isJSONString(fields["keyCode"]))
	case matchesJSONString(fields["type"], "Delay"):
		return len(fields) == 3
	default:
		return false
	}
}

func matchesJSONString(raw json.RawMessage, expected string) bool {
	raw = bytes.TrimSpace(raw)
	// Every byte of an equal decoded string needs at most six source bytes
	// (a Unicode escape). Reject unequal oversized tokens before decoding them.
	if len(raw) < 2 || raw[0] != '"' || len(raw) > 6*len(expected)+2 {
		return false
	}
	var value string
	return json.Unmarshal(raw, &value) == nil && value == expected
}

func isJSONString(raw json.RawMessage) bool {
	token := bytes.TrimSpace(raw)
	return len(token) > 0 && token[0] == '"'
}

func isJSONNumber(raw json.RawMessage) bool {
	token := bytes.TrimSpace(raw)
	return len(token) > 0 && (token[0] == '-' || token[0] >= '0' && token[0] <= '9')
}

func isJSONBoolean(raw json.RawMessage) bool {
	token := bytes.TrimSpace(raw)
	return bytes.Equal(token, []byte("true")) || bytes.Equal(token, []byte("false"))
}
