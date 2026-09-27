package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/diagnostics"
)

func TestDoctorUsage(t *testing.T) {
	fixture := newRunFixture(t)
	setRunEnvironment(t, fixture)
	for _, args := range [][]string{
		{"PRIVATE_POSITIONAL"}, {"--"}, {"--PRIVATE_FLAG"}, {"--config"}, {"--config="}, {"--config", ""},
		{"--config=a", "--config", "b"}, {"--json", "--json"}, {"--include-bindings", "--include-bindings"},
		{"--help", "--help"}, {"--json=true"}, {"--help=false"}, {"--include-bindings=false"},
		{"--help", "PRIVATE_POSITIONAL"},
	} {
		for _, jsonMode := range []bool{false, true} {
			arguments := append([]string{"doctor"}, args...)
			if jsonMode {
				arguments = append([]string{"doctor", "--json"}, args...)
			}
			reader := &observedReader{}
			var stdout, stderr bytes.Buffer
			status := run(arguments, reader, &stdout, &stderr)
			if status != 2 || reader.reads != 0 || strings.Contains(stdout.String()+stderr.String(), "PRIVATE") {
				t.Fatalf("args=%v status=%d reads=%d output=%q/%q", arguments, status, reader.reads, stdout.String(), stderr.String())
			}
			if jsonMode || slices.Contains(args, "--json") {
				report := parsedDoctorReport(t, cliOutput{status, stdout.String(), stderr.String()})
				if len(report.Checks) != 1 || report.Checks[0].Category != "command" || report.Checks[0].Code != "ERR_CLI_USAGE" || report.Checks[0].Severity != diagnostics.SeverityError {
					t.Fatalf("usage report=%+v", report)
				}
			} else if stdout.Len() != 0 || !strings.Contains(stderr.String(), "ERR_CLI_USAGE") {
				t.Fatalf("text usage=%q/%q", stdout.String(), stderr.String())
			}
		}
	}
}

func TestDoctorOutputFailure(t *testing.T) {
	fixture := newRunFixture(t)
	setRunEnvironment(t, fixture)
	for _, tc := range []struct {
		name   string
		args   []string
		stderr bool
	}{
		{"help", []string{"doctor", "--help"}, false},
		{"text report", []string{"doctor"}, false},
		{"JSON report", []string{"doctor", "--json"}, false},
		{"text usage", []string{"doctor", "--unknown"}, true},
		{"JSON usage", []string{"doctor", "--unknown", "--json"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writer := &refusingWriter{}
			var other bytes.Buffer
			var stdout, stderr io.Writer = writer, &other
			if tc.stderr {
				stdout, stderr = &other, writer
			}
			if status := run(tc.args, &observedReader{}, stdout, stderr); status != 3 || writer.calls != 1 || other.Len() != 0 {
				t.Fatalf("status=%d writes=%d other=%q", status, writer.calls, other.String())
			}
		})
	}
	t.Run("broken pipe", func(t *testing.T) {
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
	})
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
