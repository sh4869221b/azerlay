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

func runProfiles(jsonMode, help bool, stdout, stderr io.Writer) int {
	report := operationReport{SchemaVersion: 1, Command: "profiles show"}
	switch {
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
