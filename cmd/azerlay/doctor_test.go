package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/diagnostics"
)

// This is the original TestDoctorOutputFailure/broken_pipe subprocess case.
func TestCLIDoctorBrokenPipe(t *testing.T) {
	fixture := newRunFixture(t)
	setRunEnvironment(t, fixture)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, cliBinary, "doctor", "--json")
	cmd.Env, cmd.Stdout = fixture.env, writer
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	var exit *exec.ExitError
	if ctx.Err() != nil || !errors.As(err, &exit) || exit.ExitCode() != 3 || stderr.Len() != 0 {
		t.Fatalf("broken pipe exit=%v context=%v stderr=%q", err, ctx.Err(), stderr.String())
	}
}

func parsedDoctorReport(t *testing.T, output cliOutput) diagnostics.Report {
	t.Helper()
	parsedJSON(t, output.stdout)
	decoder := json.NewDecoder(strings.NewReader(output.stdout))
	decoder.DisallowUnknownFields()
	var report diagnostics.Report
	if err := decoder.Decode(&report); err != nil {
		t.Fatal(err)
	}
	if output.stderr != "" || !strings.HasSuffix(output.stdout, "\n") || report.SchemaVersion != 2 || report.Command != "doctor" || report.ExitCode != output.status {
		t.Fatalf("invalid doctor report: %+v", output)
	}
	return report
}

func requireDoctorCode(t *testing.T, report diagnostics.Report, code string) {
	t.Helper()
	for _, check := range report.Checks {
		if check.Code == code {
			return
		}
	}
	t.Fatalf("missing %s: %+v", code, report.Checks)
}

func requireDoctorNoWrites(t *testing.T, root string, before map[string]cliFileState) {
	t.Helper()
	if !reflect.DeepEqual(before, cliTree(t, root)) {
		t.Fatal("doctor changed files, entries or modes")
	}
}

func doctorRoot(fixture runFixture) string {
	return filepath.Dir(filepath.Dir(filepath.Dir(fixture.config)))
}
