package gameprofile

import (
	"strings"

	"github.com/sh4869221b/azerlay/internal/profile"
)

type Resolved struct {
	Label          string
	BindingDisplay string
}

func Resolve(control profile.ControlBinding, binding profile.TriggerBinding, game Definition) Resolved {
	display := bindingDisplay(binding)
	result := Resolved{Label: display, BindingDisplay: display}
	identity := control.SourceIdentity
	if !identity.Invalid && identity.InputID != nil && *identity.InputID > 0 {
		if label := game.Controls[ControlKey{InputID: *identity.InputID, Trigger: binding.Trigger}]; strings.TrimSpace(label) != "" {
			result.Label = label
			return result
		}
	}
	if control.Label != nil && *control.Label != "" {
		result.Label = *control.Label
		return result
	}
	var code profile.CanonicalCode
	switch {
	case binding.Kind == profile.BindingKeyboard && len(binding.Actions) == 1 && binding.Actions[0].Kind == profile.ActionKeyboard && len(binding.Actions[0].Modifiers) == 0:
		code = binding.Actions[0].Code
	case binding.Kind == profile.BindingTurbo && binding.Turbo != nil:
		code = binding.Turbo.Code
	}
	if code != "" {
		if label := game.Bindings[code]; strings.TrimSpace(label) != "" {
			result.Label = label
		}
	}
	return result
}
