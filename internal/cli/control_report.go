package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/sh4869221b/azerlay/internal/control"
)

type controlReport struct {
	SchemaVersion int            `json:"schema_version"`
	Command       string         `json:"command"`
	OK            bool           `json:"ok"`
	Result        control.Result `json:"result"`
	Error         *ReportError   `json:"error"`
}

func writeControlReport(report controlReport, jsonMode bool, stdout, stderr io.Writer) int {
	status := 0
	if report.Error != nil {
		status = 1
		if report.Error.Stage == "usage" {
			status = 2
		}
	}
	if jsonMode {
		if err := json.NewEncoder(stdout).Encode(report); err != nil {
			return 1
		}
		return status
	}
	if report.Error != nil {
		if _, err := fmt.Fprintf(stderr, "%s (%s): %s\n%s\n", report.Error.Code, report.Error.Stage, report.Error.Summary, report.Error.Remediation); err != nil {
			return 1
		}
		return status
	}
	if _, err := io.WriteString(stdout, renderControlReport(report.Result)); err != nil {
		return 1
	}
	return status
}

func renderControlReport(result control.Result) string {
	var text strings.Builder
	switch value := result.(type) {
	case control.VisibilityResult:
		fmt.Fprintf(&text, "Requested visibility: %t\n", value.Visible)
	case control.ReloadResult:
		fmt.Fprintf(&text, "Reload accepted: %t\nRequest generation: %d\nCheck status for the reload result.\n", value.Accepted, value.RequestGeneration)
	case control.SelectionResult:
		renderActiveProfile(&text, &value.ActiveProfile)
		fmt.Fprintf(&text, "Runtime generation: %d\nSaved selection is unchanged.\n", value.Generation)
	case control.QuitResult:
		fmt.Fprintf(&text, "Quitting: %t\n", value.Quitting)
	case control.Status:
		fmt.Fprintf(&text, "Status schema: %d\nUptime: %.3f seconds\nRequested visibility: %t\nRuntime generation: %d\n", value.SchemaVersion, value.UptimeSeconds, value.Visible, value.Generation)
		if value.Overlay != nil {
			fmt.Fprintf(&text, "Overlay mapped: %t\nInput region applied: %t\n", value.Overlay.Mapped, value.Overlay.InputRegionApplied)
		}
		renderActiveProfile(&text, value.ActiveProfile)
		device := "unavailable"
		if value.Device != nil {
			device = *value.Device
		}
		fmt.Fprintf(&text, "Device: %s\nEvent nodes: %v\nEvent rate: %s\nDropped count: %s\nResync count: %s\nRender rate: %s\n", device, value.EventNodes, formatRate(value.EventRate), formatCount(value.DroppedCount), formatCount(value.ResyncCount), formatRate(value.RenderRate))
		fmt.Fprintf(&text, "Reload request generation: %d\nConfig generation: %d\n", value.LastReload.RequestGeneration, value.LastReload.ConfigGeneration)
		for _, item := range []struct {
			name       string
			diagnostic *control.Diagnostic
		}{{"Config failure", value.LastReload.ConfigFailure}, {"Watch failure", value.LastReload.WatchFailure}} {
			if item.diagnostic == nil {
				fmt.Fprintf(&text, "%s: none\n", item.name)
			} else {
				fmt.Fprintf(&text, "%s: %s (%s): %s\n", item.name, item.diagnostic.Code, item.diagnostic.Stage, item.diagnostic.Reason)
			}
		}
		for _, reason := range value.DegradedReasons {
			fmt.Fprintf(&text, "Degraded: %s (%s): %s\n", reason.Code, reason.Stage, reason.Reason)
		}
	}
	return text.String()
}

func formatRate(value *float64) string {
	if value == nil {
		return "unavailable"
	}
	return strconv.FormatFloat(*value, 'f', 3, 64)
}
func formatCount(value *uint64) string {
	if value == nil {
		return "unavailable"
	}
	return strconv.FormatUint(*value, 10)
}

func renderActiveProfile(text *strings.Builder, active *control.ProfileStatus) {
	if active == nil {
		text.WriteString("Active profile: unavailable\n")
		return
	}
	name := "<unnamed>"
	if active.Name != nil {
		name = strconv.Quote(*active.Name)
	}
	fmt.Fprintf(text, "Active profile: %d. %s\nSource: %s\nSource reference: %s\n", active.ProfileIndex, name, active.Source, active.SourceRef)
}
