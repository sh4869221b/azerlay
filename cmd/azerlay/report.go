package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/sh4869221b/azerlay/internal/profile"
)

// CLI schema 1 is independent of the normalized model's schema version.
// These DTOs deliberately cannot carry raw exports or binding details.
type operationReport struct {
	SchemaVersion int            `json:"schema_version"`
	Command       string         `json:"command"`
	OK            bool           `json:"ok"`
	Result        *profileReport `json:"result"`
	Error         *reportError   `json:"error"`
}

type reportError struct {
	Code        string `json:"code"`
	Stage       string `json:"stage"`
	Summary     string `json:"summary"`
	Remediation string `json:"remediation"`
}

type profileReport struct {
	RootKind             profile.RootKind `json:"root_kind"`
	ExportVersion        exportVersion    `json:"export_version"`
	Profiles             []profileRow     `json:"profiles"`
	SelectedProfileIndex *int             `json:"selected_profile_index"`
	Warnings             []profileWarning `json:"warnings"`
}

type exportVersion struct {
	Present bool            `json:"present"`
	Value   json.RawMessage `json:"value"`
}

type profileRow struct {
	Index      int     `json:"index"`
	Name       *string `json:"name"`
	InputCount int     `json:"input_count"`
}

type profileWarning struct {
	Code         string `json:"code"`
	ProfileIndex int    `json:"profile_index"`
	Count        int    `json:"count"`
}

func projectReport(bundle profile.ProfileBundle) *profileReport {
	result := &profileReport{
		RootKind: bundle.RootKind,
		Profiles: make([]profileRow, 0, len(bundle.Profiles)),
		Warnings: make([]profileWarning, 0),
	}
	var version json.RawMessage
	switch bundle.RootKind {
	case profile.RootBundle:
		version = bundle.Raw.Version
	case profile.RootSingle:
		version = bundle.Profiles[0].Raw.Version
	}
	result.ExportVersion = exportVersion{Present: version != nil, Value: version}
	for index, item := range bundle.Profiles {
		result.Profiles = append(result.Profiles, profileRow{index + 1, item.Name, len(item.Controls)})
		unknown := 0
		for _, control := range item.Controls {
			for _, binding := range control.Bindings {
				if binding.Kind == profile.BindingUnknown {
					unknown++
				}
			}
		}
		if unknown > 0 {
			result.Warnings = append(result.Warnings, profileWarning{"WARN_IMPORT_UNKNOWN_BINDINGS", index + 1, unknown})
		}
	}
	return result
}

func writeReport(report operationReport, jsonMode bool, stdout, stderr io.Writer) int {
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
	if report.Result != nil {
		if _, err := io.WriteString(stdout, renderTextReport(report.Result)); err != nil {
			return 1
		}
	}
	if report.Error != nil {
		if _, err := fmt.Fprintf(stderr, "%s (%s): %s\n%s\n", report.Error.Code, report.Error.Stage, report.Error.Summary, report.Error.Remediation); err != nil {
			return 1
		}
	}
	return status
}

func renderTextReport(result *profileReport) string {
	var text strings.Builder
	version := "<missing>"
	if result.ExportVersion.Present {
		version = string(result.ExportVersion.Value)
		var value string
		if json.Unmarshal(result.ExportVersion.Value, &value) == nil && version != "null" {
			version = strconv.Quote(value)
		}
	}
	fmt.Fprintf(&text, "Root kind: %s\nExport version: %s\nProfiles: %d\n", result.RootKind, version, len(result.Profiles))
	for _, row := range result.Profiles {
		name := "<unnamed>"
		if row.Name != nil {
			name = strconv.Quote(*row.Name)
		}
		fmt.Fprintf(&text, "  %d. %s (%d inputs)\n", row.Index, name, row.InputCount)
	}
	if result.SelectedProfileIndex != nil {
		fmt.Fprintf(&text, "Selected profile: %d\n", *result.SelectedProfileIndex)
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(&text, "%s: profile %d, %d Unknown outcomes\n", warning.Code, warning.ProfileIndex, warning.Count)
	}
	text.WriteString("No state was saved.\n")
	return text.String()
}
