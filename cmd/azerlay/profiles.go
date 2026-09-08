package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/sh4869221b/azerlay/internal/profilesource"
)

const profilesHelp = `Usage: azerlay profiles show [--json]

Show the saved profile selection without supplying the original input.

Options:
  --json  Print schema-versioned JSON
  --help  Print this help without accessing storage

No state is saved. Missing or incompatible caches are recovered in memory.`

func runProfiles(args []string, stdout, stderr io.Writer) int {
	jsonMode, help := false, false
	valid := len(args) > 0 && args[0] == "show"
	for i, arg := range args {
		if i == 0 && arg == "show" {
			continue
		}
		switch arg {
		case "--json":
			valid = valid && !jsonMode
			jsonMode = true
		case "--help":
			valid = valid && !help
			help = true
		default:
			valid = false
		}
	}
	// The group help is the same minimal surface as show help.
	if len(args) == 1 && args[0] == "--help" {
		valid = true
	}
	report := operationReport{SchemaVersion: 1, Command: "profiles show"}
	switch {
	case !valid:
		report.Error = &reportError{"ERR_CLI_USAGE", "usage", "Invalid command arguments.", "Use profiles show --help; no input, release or selector is accepted."}
	case help:
		if _, err := fmt.Fprintln(stdout, profilesHelp); err != nil {
			return 1
		}
		return 0
	default:
		dataHome, err := profilesource.ResolveDataHome()
		if err != nil {
			report.Error = storageFailure(err)
			break
		}
		source := profilesource.NewImportedSource(dataHome)
		ctx := context.Background()
		selected, err := source.Selected(ctx)
		if err != nil {
			report.Error = storageFailure(err)
			break
		}
		bundle, err := source.Load(ctx, selected.Source)
		if err != nil {
			report.Error = storageFailure(err)
			break
		}
		report.Result = projectReport(*bundle)
		report.Result.SelectedProfileIndex = &selected.ProfileIndex
		report.OK = true
	}
	return writeReport(report, jsonMode, stdout, stderr)
}

func storageFailure(err error) *reportError {
	var failure *profilesource.Error
	errors.As(err, &failure) // All source API errors have this privacy-safe type.
	switch failure.Code {
	case profilesource.ERR_PROFILE_NOT_FOUND:
		return &reportError{failure.Code, "selection", "No saved profile selection was found.", "Import a supported export and select a profile first."}
	default:
		return &reportError{profilesource.ERR_PROFILE_STORAGE, "storage", "Profile storage could not be accessed.", "Check the data directory and saved source integrity before retrying."}
	}
}
