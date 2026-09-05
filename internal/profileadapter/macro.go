package profileadapter

import (
	"bytes"
	"encoding/json"

	"github.com/sh4869221b/azerlay/internal/profileraw"
)

const maxRecognizedMacroSteps = 1000

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
	var steps []json.RawMessage
	if json.Unmarshal(stepsToken, &steps) != nil {
		return false
	}
	for _, step := range steps {
		if !isRecognizedMacroStep(step) {
			return false
		}
	}
	return len(steps) > maxRecognizedMacroSteps
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
