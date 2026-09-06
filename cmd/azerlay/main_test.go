package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestRun_WhenVersionCommand(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"version"}, &observedReader{}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}
	if stdout.String() != "azerlay dev\n" {
		t.Fatalf("stdout = %q, want exact version line", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

type refusingWriter struct{ calls int }

func (w *refusingWriter) Write([]byte) (int, error) {
	w.calls++
	return 0, errors.New("PRIVATE_WRITER")
}

func TestRunOutputFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		args   []string
		stderr bool
	}{
		{"version", []string{"version"}, false},
		{"root help", []string{"--help"}, false},
		{"command help", []string{"validate", "--help"}, false},
		{"unknown", []string{"unknown"}, true},
		{"text success", []string{"validate", "--software-release", "2.0.2", "--text", fixtureText}, false},
		{"json success", []string{"validate", "--json", "--software-release", "2.0.2", "--text", fixtureText}, false},
		{"json decode error", []string{"validate", "--json", "--text", "{"}, false},
		{"text decode error", []string{"validate", "--text", "{"}, true},
		{"json usage", []string{"validate", "--json"}, false},
		{"text usage", []string{"validate"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given: only the real output sink refuses writes.
			writer := &refusingWriter{}
			var other bytes.Buffer
			var stdout, stderr io.Writer = writer, &other
			if tc.stderr {
				stdout, stderr = &other, writer
			}
			// When
			status := run(tc.args, &observedReader{}, stdout, stderr)
			// Then: no recursive fallback or misleading success on the other sink.
			if status != 1 || writer.calls != 1 || other.Len() != 0 {
				t.Fatalf("status=%d writes=%d other=%q", status, writer.calls, other.String())
			}
		})
	}
}

func TestRun_WhenHelpRequested(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"--help"}, &observedReader{}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}
	if stdout.Len() == 0 {
		t.Fatal("stdout is empty")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_WhenCommandIsUnknown(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"unknown"}, &observedReader{}, &stdout, &stderr)

	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr is empty")
	}
}

func TestRunDoesNotReadInput(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"version"}, {"--help"}, {"validate", "--help"},
		{"validate", "--help", "--json"}, {"validate", "--json", "--help"},
	} {
		t.Run(strings.Join(args, "/"), func(t *testing.T) {
			// Given
			reader := &observedReader{}
			var stdout, stderr bytes.Buffer
			// When
			status := run(args, reader, &stdout, &stderr)
			// Then: help remains plain text even with --json.
			if status != 0 || reader.reads != 0 || stdout.Len() == 0 || stderr.Len() != 0 || strings.HasPrefix(stdout.String(), "{") {
				t.Fatalf("status=%d reads=%d stdout=%q stderr=%q", status, reader.reads, stdout.String(), stderr.String())
			}
		})
	}
}
