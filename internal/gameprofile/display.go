package gameprofile

import (
	"strconv"
	"strings"

	"github.com/sh4869221b/azerlay/internal/profile"
)

func bindingDisplay(binding profile.TriggerBinding) string {
	switch binding.Kind {
	case profile.BindingKeyboard:
		parts := make([]string, len(binding.Actions))
		for i, action := range binding.Actions {
			keys := make([]string, 0, len(action.Modifiers)+1)
			for _, modifier := range action.Modifiers {
				keys = append(keys, displayKey(modifier))
			}
			keys = append(keys, displayKey(action.Code))
			parts[i] = strings.Join(keys, "+")
		}
		return strings.Join(parts, ", ")
	case profile.BindingTurbo:
		return "Turbo (" + displayKey(binding.Turbo.Code) + ", " + strconv.Itoa(binding.Turbo.ClicksPerSecond) + "/s)"
	case profile.BindingMacro:
		if binding.Macro.RepeatWhileHeld {
			return "Macro (repeat while held)"
		}
		return "Macro"
	case profile.BindingStick:
		angle := strconv.Itoa(binding.Stick.AngleDegrees) + " deg)"
		if binding.Stick.Mode == profile.StickModeXbox {
			return "Left Stick (Xbox, " + angle
		}
		directions := binding.Stick.KeyboardDirections
		return "Stick (Up: " + displayKey(directions.Up) + ", Right: " + displayKey(directions.Right) + ", Down: " + displayKey(directions.Down) + ", Left: " + displayKey(directions.Left) + ", " + angle
	case profile.BindingUnbound:
		return "Unbound"
	case profile.BindingUnknown:
		if binding.Unknown == nil {
			return "Unknown"
		}
		if binding.Unknown.RawDisplay != "" {
			return "Unknown (" + binding.Unknown.RawDisplay + ")"
		}
		return "Unknown (" + binding.Unknown.Reason + ")"
	default:
		return "Unknown"
	}
}

func displayKey(code profile.CanonicalCode) string {
	switch code {
	case "KEY_LEFTCTRL":
		return "Left Ctrl"
	case "KEY_RIGHTCTRL":
		return "Right Ctrl"
	case "KEY_LEFTSHIFT":
		return "Left Shift"
	case "KEY_RIGHTSHIFT":
		return "Right Shift"
	case "KEY_LEFTALT":
		return "Left Alt"
	case "KEY_RIGHTALT":
		return "Right Alt"
	case "KEY_LEFTMETA":
		return "Left Meta"
	case "KEY_RIGHTMETA":
		return "Right Meta"
	}
	name := string(code)
	if strings.HasPrefix(name, "KEY_") {
		return strings.TrimPrefix(name, "KEY_")
	}
	if strings.HasPrefix(name, "BTN_") {
		return "Button " + strings.TrimPrefix(name, "BTN_")
	}
	return name
}
