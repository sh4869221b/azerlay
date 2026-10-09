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
	result := Resolved{BindingDisplay: display}
	identity := control.SourceIdentity
	if !identity.Invalid && identity.InputID != nil && *identity.InputID > 0 {
		if label := game.Controls[ControlKey{InputID: *identity.InputID, Trigger: binding.Trigger}]; strings.TrimSpace(label) != "" {
			result.Label = label
			return result
		}
	}
	if control.Label != nil {
		result.Label = *control.Label
	}
	return result
}
