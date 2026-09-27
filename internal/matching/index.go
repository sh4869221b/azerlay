package matching

import (
	"slices"

	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/profile"
)

func Build(selected profile.Profile, source profile.SourceMetadata, definition layout.Definition, context Context) Index {
	supported := source.SoftwareRelease == "2.0.2" && source.SourceScope == "azeron-software-export" &&
		fixedScope(definition.Model, definition.Hand, definition.Applicability) &&
		fixedScope(context.Model, context.Hand, context.Applicability)
	index := Index{ScopeSupported: supported, Controls: make([]Control, len(selected.Controls))}
	byID := make(map[int]layout.Control, len(definition.Controls))
	if supported {
		for _, control := range definition.Controls {
			byID[control.SourceInputID] = control
		}
	}
	byCode := make(map[profile.CanonicalCode][]Candidate)
	for controlIndex, sourceControl := range selected.Controls {
		control := Control{ControlIndex: controlIndex, MappingState: MappingUnresolved, Bindings: make([]Binding, len(sourceControl.Bindings))}
		identity := sourceControl.SourceIdentity
		if !identity.Invalid && identity.InputID != nil {
			if mapped, ok := byID[*identity.InputID]; ok &&
				(identity.PinOne == nil || *identity.PinOne == mapped.PinOne) &&
				(identity.PinTwo == nil || *identity.PinTwo == mapped.PinTwo) {
				control.RegionID = mapped.ID
				control.MappingState = MappingMapped
			}
		}
		for bindingIndex, binding := range sourceControl.Bindings {
			control.Bindings[bindingIndex] = Binding{Trigger: binding.Trigger, Kind: binding.Kind}
			if binding.Kind == profile.BindingUnknown {
				index.HasUnknownBindings = true
			}
			seen := make(map[profile.CanonicalCode]bool)
			for _, code := range bindingCodes(binding) {
				if seen[code] {
					continue
				}
				seen[code] = true
				byCode[code] = append(byCode[code], Candidate{
					ControlIndex: controlIndex, BindingIndex: bindingIndex,
					Trigger: binding.Trigger, Kind: binding.Kind,
					RegionID: control.RegionID, MappingState: control.MappingState,
				})
			}
		}
		index.Controls[controlIndex] = control
	}
	codes := make([]profile.CanonicalCode, 0, len(byCode))
	for code := range byCode {
		codes = append(codes, code)
	}
	slices.Sort(codes)
	index.Outputs = make([]Output, len(codes))
	for i, code := range codes {
		candidates := byCode[code]
		firstControl := -1
		output := Output{Code: code, Candidates: candidates}
		for _, candidate := range candidates {
			if firstControl == -1 {
				firstControl = candidate.ControlIndex
			} else if firstControl != candidate.ControlIndex {
				output.Ambiguous = true
			}
			if candidate.MappingState == MappingUnresolved {
				output.HasUnresolvedCandidates = true
			}
		}
		index.Outputs[i] = output
	}
	return index
}

func fixedScope(model, hand string, applicability layout.Applicability) bool {
	return model == "cyborg-ii" && hand == "left" && applicability.SoftwareRelease == "2.0.2" &&
		applicability.DisplayedFirmware == "111" && applicability.HardwareRevision == nil && applicability.Mode == "keyboard-stick"
}

func bindingCodes(binding profile.TriggerBinding) []profile.CanonicalCode {
	switch binding.Kind {
	case profile.BindingKeyboard:
		var codes []profile.CanonicalCode
		for _, action := range binding.Actions {
			codes = append(codes, action.Code)
			codes = append(codes, action.Modifiers...)
		}
		return codes
	case profile.BindingTurbo:
		if binding.Turbo != nil {
			return []profile.CanonicalCode{binding.Turbo.Code}
		}
	case profile.BindingMacro:
		if binding.Macro != nil {
			var codes []profile.CanonicalCode
			for _, step := range binding.Macro.Steps {
				if step.Kind == profile.MacroStepButton {
					codes = append(codes, step.Code)
				}
			}
			return codes
		}
	case profile.BindingStick:
		if binding.Stick != nil && binding.Stick.Mode == profile.StickModeKeyboard {
			directions := binding.Stick.KeyboardDirections
			return []profile.CanonicalCode{directions.Up, directions.Right, directions.Down, directions.Left}
		}
	}
	return nil
}
