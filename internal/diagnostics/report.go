package diagnostics

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func WriteReport(w io.Writer, report Report, jsonMode bool) error {
	var output []byte
	if jsonMode {
		encoded, err := json.Marshal(report)
		if err != nil {
			return err
		}
		output = append(encoded, '\n')
	} else {
		output = []byte(renderText(report))
	}
	n, err := w.Write(output)
	if err == nil && n != len(output) {
		return io.ErrShortWrite
	}
	return err
}

func renderText(report Report) string {
	var text strings.Builder
	fmt.Fprintf(&text, "Doctor diagnostics (exit %d)\n", report.ExitCode)
	for _, check := range report.Checks {
		fmt.Fprintf(&text, "[%s] %s %s: %s\n", check.Severity, check.Category, check.Code, check.Summary)
		if check.Target != nil {
			fmt.Fprintf(&text, "  Target: %s\n", strconv.Quote(*check.Target))
		}
		if check.Remediation != "" {
			fmt.Fprintf(&text, "  Remediation: %s\n", check.Remediation)
		}
	}
	if report.ProfileDetails != nil {
		fmt.Fprintf(&text, "Profile details: source ordinal %d\n", report.ProfileDetails.ProfileIndex)
		for _, control := range report.ProfileDetails.Controls {
			label := "<unlabeled>"
			if control.Label != nil {
				label = strconv.Quote(*control.Label)
			}
			fmt.Fprintf(&text, "  Input %d: %s\n", control.InputIndex, label)
			for _, binding := range control.Bindings {
				fmt.Fprintf(&text, "    %s: %s\n", binding.Trigger, binding.Kind)
				for _, action := range binding.Actions {
					fmt.Fprintf(&text, "      Action: %s %s; modifiers: %v\n", action.Kind, action.Code, action.Modifiers)
				}
				if binding.TriggerDelayMS != nil {
					fmt.Fprintf(&text, "      Trigger delay (ms): %d\n", *binding.TriggerDelayMS)
				}
				if binding.TriggerIntervalMS != nil {
					fmt.Fprintf(&text, "      Trigger interval (ms): %d\n", *binding.TriggerIntervalMS)
				}
				if binding.ReleaseBehavior != nil {
					fmt.Fprintf(&text, "      Release behavior: %s\n", strconv.Quote(*binding.ReleaseBehavior))
				}
				if binding.UnknownReason != nil {
					fmt.Fprintf(&text, "      Unknown reason: %s\n", strconv.Quote(*binding.UnknownReason))
				}
			}
		}
	}
	return text.String()
}
