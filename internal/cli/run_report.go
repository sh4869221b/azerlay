package cli

import (
	"io"

	"github.com/sh4869221b/azerlay/internal/control"
)

// WriteRunResult writes native startup's existing control result as text.
func WriteRunResult(result control.Result, stdout, stderr io.Writer) int {
	return writeControlReport(controlReport{SchemaVersion: 1, Command: "run", OK: true, Result: result}, false, stdout, stderr)
}

// WriteRunFailure writes a classified, privacy-safe native startup failure.
// Native errors are classified by the executable, not by this package.
func WriteRunFailure(failure *ReportError, stdout, stderr io.Writer) int {
	return writeControlReport(controlReport{SchemaVersion: 1, Command: "run", Error: failure}, false, stdout, stderr)
}
