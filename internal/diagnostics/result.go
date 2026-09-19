package diagnostics

import (
	"slices"

	"github.com/sh4869221b/azerlay/internal/profile"
)

type Severity string

const (
	SeverityOK            Severity = "ok"
	SeverityWarning       Severity = "warning"
	SeverityError         Severity = "error"
	SeverityInternalError Severity = "internal_error"
)

type Check struct {
	Category    string   `json:"category"`
	Code        string   `json:"code"`
	Summary     string   `json:"summary"`
	Target      *string  `json:"target"`
	Remediation string   `json:"remediation"`
	Severity    Severity `json:"severity"`
}

type Report struct {
	SchemaVersion  int             `json:"schema_version"`
	Command        string          `json:"command"`
	ExitCode       int             `json:"exit_code"`
	Checks         []Check         `json:"checks"`
	ProfileDetails *ProfileDetails `json:"profile_details,omitempty"`
}

type ProfileDetails struct {
	ProfileIndex int             `json:"profile_index"`
	Controls     []controlDetail `json:"controls"`
}

type controlDetail struct {
	InputIndex int             `json:"input_index"`
	Label      *string         `json:"label"`
	Bindings   []bindingDetail `json:"bindings"`
}

type bindingDetail struct {
	Trigger           profile.TriggerKind `json:"trigger"`
	Kind              profile.BindingKind `json:"kind"`
	Actions           []actionDetail      `json:"actions"`
	TriggerDelayMS    *int                `json:"trigger_delay_ms"`
	TriggerIntervalMS *int                `json:"trigger_interval_ms"`
	ReleaseBehavior   *string             `json:"release_behavior"`
	UnknownReason     *string             `json:"unknown_reason"`
}

type actionDetail struct {
	Kind      profile.ActionKind      `json:"kind"`
	Code      profile.CanonicalCode   `json:"code"`
	Modifiers []profile.CanonicalCode `json:"modifiers"`
}

func NewReport(checks []Check, details *ProfileDetails) Report {
	ordered := append([]Check{}, checks...)
	categories := []string{
		"command", "session", "libraries", "layer-shell", "monitor", "configuration",
		"profile-source", "device-discovery", "permissions", "evdev-capabilities", "control-socket",
	}
	slices.SortStableFunc(ordered, func(a, b Check) int {
		return slices.Index(categories, a.Category) - slices.Index(categories, b.Category)
	})
	exitCode := 0
	for _, check := range ordered {
		exitCode = max(exitCode, severityExitCode(check.Severity))
	}
	return Report{SchemaVersion: 1, Command: "doctor", ExitCode: exitCode, Checks: ordered, ProfileDetails: details}
}

func severityExitCode(severity Severity) int {
	switch severity {
	case SeverityOK:
		return 0
	case SeverityWarning:
		return 1
	case SeverityError:
		return 2
	case SeverityInternalError:
		return 3
	default:
		return 3
	}
}

// The caller must require this invocation's explicit disclosure option.
func ProjectProfileDetails(profileIndex int, selected profile.Profile) *ProfileDetails {
	details := &ProfileDetails{ProfileIndex: profileIndex, Controls: make([]controlDetail, 0, len(selected.Controls))}
	for index, control := range selected.Controls {
		row := controlDetail{InputIndex: index + 1, Label: control.Label, Bindings: make([]bindingDetail, 0, len(control.Bindings))}
		for _, binding := range control.Bindings {
			item := bindingDetail{
				Trigger: binding.Trigger, Kind: binding.Kind,
				Actions:        make([]actionDetail, 0, len(binding.Actions)),
				TriggerDelayMS: binding.TriggerDelayMS, TriggerIntervalMS: binding.TriggerIntervalMS,
				ReleaseBehavior: binding.ReleaseBehavior,
			}
			if binding.Unknown != nil {
				item.UnknownReason = &binding.Unknown.Reason
			}
			for _, action := range binding.Actions {
				item.Actions = append(item.Actions, actionDetail{
					Kind: action.Kind, Code: action.Code,
					Modifiers: append([]profile.CanonicalCode{}, action.Modifiers...),
				})
			}
			row.Bindings = append(row.Bindings, item)
		}
		details.Controls = append(details.Controls, row)
	}
	return details
}
