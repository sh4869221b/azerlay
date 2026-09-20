package profileadapter

import (
	"encoding/json"
	"slices"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

const unknownReasonUnmapped = "unmapped_binding"

var recognizedTypeArrays = [][]string{
	{"1", "1", "1"},
	{"1", "11", "11"},
	{"11", "1", "11"},
	{"11", "11", "1"},
	{"11", "11", "11"},
	{"16", "11", "11"},
	{"4", "11", "11"},
	{"21", "11", "11"},
}

func preserveProfile(raw profileraw.RawProfile, rootKind profile.RootKind, profileIndex int) profile.Profile {
	controls := make([]profile.ControlBinding, len(raw.Inputs))
	for inputIndex, input := range raw.Inputs {
		controls[inputIndex] = preserveControl(input, rootKind, profileIndex, inputIndex)
	}
	return profile.Profile{
		ID:       scalarString(raw.ID),
		Name:     scalarString(raw.Name),
		Controls: controls,
		Raw: profile.RawProfileReference{
			ID:      cloneScalarRaw(raw.ID),
			Name:    cloneScalarRaw(raw.Name),
			Version: cloneScalarRaw(raw.Version),
			Unknown: cloneRawMap(raw.Unknown),
		},
	}
}

func preserveControl(raw profileraw.RawInput, rootKind profile.RootKind, profileIndex, inputIndex int) profile.ControlBinding {
	return profile.ControlBinding{
		Label:    decodeLabel(raw["label"]),
		Bindings: unknownBindings(raw["types"]),
		Raw: profile.RawBindingReference{
			RootKind:     rootKind,
			ProfileIndex: profileIndex,
			InputIndex:   inputIndex,
			Fields:       cloneRawMap(raw),
		},
	}
}

func unknownBindings(rawTypes json.RawMessage) []profile.TriggerBinding {
	triggers := []profile.TriggerKind{profile.TriggerUnknown}
	var types []string
	if json.Unmarshal(rawTypes, &types) == nil {
		for _, recognized := range recognizedTypeArrays {
			if slices.Equal(types, recognized) {
				triggers = []profile.TriggerKind{profile.TriggerSingle, profile.TriggerLong, profile.TriggerDouble}
				break
			}
		}
	}
	bindings := make([]profile.TriggerBinding, len(triggers))
	for index, trigger := range triggers {
		bindings[index] = profile.TriggerBinding{
			Trigger: trigger,
			Kind:    profile.BindingUnknown,
			Unknown: &profile.UnknownBinding{Reason: unknownReasonUnmapped},
		}
	}
	return bindings
}

func decodeLabel(raw json.RawMessage) *string {
	if raw == nil {
		return nil
	}
	var value *string
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	return value
}

func scalarString(raw *profileraw.RawScalar) *string {
	if raw == nil || raw.Kind != profileraw.ScalarString {
		return nil
	}
	value := raw.String
	return &value
}

func cloneScalarRaw(raw *profileraw.RawScalar) json.RawMessage {
	if raw == nil {
		return nil
	}
	return cloneRaw(raw.Raw)
}

func cloneRawMap(raw map[string]json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	cloned := make(map[string]json.RawMessage, len(raw))
	for key, value := range raw {
		cloned[key] = cloneRaw(value)
	}
	return cloned
}

func cloneRaw(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}
