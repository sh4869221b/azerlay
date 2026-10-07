package cli

import (
	"bytes"
	"io"
	"testing"

	"github.com/sh4869221b/azerlay/internal/control"
)

func TestWriteRunReports(t *testing.T) {
	t.Parallel()
	t.Run("result", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		status := WriteRunResult(control.VisibilityResult{Visible: true}, &stdout, &stderr)
		if status != 0 || stdout.String() != "Requested visibility: true\n" || stderr.Len() != 0 {
			t.Fatalf("result status=%d stdout=%q stderr=%q", status, &stdout, &stderr)
		}
		writer := &refusingWriter{}
		if status := WriteRunResult(control.VisibilityResult{}, writer, io.Discard); status != 1 || writer.calls != 1 {
			t.Fatalf("result output failure status=%d writes=%d", status, writer.calls)
		}
	})
	t.Run("failure", func(t *testing.T) {
		failure := &ReportError{Code: "ERR_RUNTIME_WAYLAND", Stage: "runtime", Summary: "Wayland display is unavailable.", Remediation: "Use a Wayland session."}
		var stdout, stderr bytes.Buffer
		status := WriteRunFailure(failure, &stdout, &stderr)
		if status != 1 || stdout.Len() != 0 || stderr.String() != "ERR_RUNTIME_WAYLAND (runtime): Wayland display is unavailable.\nUse a Wayland session.\n" {
			t.Fatalf("failure status=%d stdout=%q stderr=%q", status, &stdout, &stderr)
		}
		writer := &refusingWriter{}
		if status := WriteRunFailure(failure, io.Discard, writer); status != 1 || writer.calls != 1 {
			t.Fatalf("failure output failure status=%d writes=%d", status, writer.calls)
		}
	})
}
