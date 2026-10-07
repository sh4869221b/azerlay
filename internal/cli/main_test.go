package cli

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profilesource"
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

func TestImportStorageFailure(t *testing.T) {
	// Given: a real file obstructs the application directory.
	root := isolateCLI(t)
	t.Setenv("XDG_DATA_HOME", root)
	cliWrite(t, filepath.Join(root, "azerlay"), []byte("PRIVATE_FIELD"), 0600)
	before := cliTree(t, root)
	var stdout, stderr bytes.Buffer
	// When
	status := run([]string{"import", "--json", "--software-release", "2.0.2", "--text", fixtureText}, &observedReader{}, &stdout, &stderr)
	// Then: a failed commit cannot report a selected or partial profile.
	checkCLIFailure(t, cliOutput{status, stdout.String(), stderr.String()}, 1, "import", "ERR_PROFILE_STORAGE", "storage", "null")
	if !reflect.DeepEqual(before, cliTree(t, root)) {
		t.Fatal("failed import changed obstructed storage")
	}
}

func TestImportOutputFailureAfterCommit(t *testing.T) {
	// Given: storage works but the actual output sink rejects delivery.
	root := isolateCLI(t)
	writer := &refusingWriter{}
	var stderr bytes.Buffer
	// When
	status := run([]string{"import", "--json", "--software-release", "2.0.2", "--text", fixtureText}, &observedReader{}, writer, &stderr)
	// Then: output failure exits 1 without rolling back the committed selection.
	if status != 1 || writer.calls != 1 || stderr.Len() != 0 {
		t.Fatalf("status=%d writes=%d stderr=%q", status, writer.calls, &stderr)
	}
	source := profilesource.NewImportedSource(filepath.Join(root, "data"))
	selected, err := source.Selected(t.Context())
	if err != nil || selected.ProfileIndex != 1 {
		t.Fatalf("selection after output failure=%+v error=%v", selected, err)
	}
	bundle, err := source.Load(t.Context(), selected.Source)
	if err != nil || len(bundle.Profiles) != 1 || bundle.Profiles[0].ID == nil || *bundle.Profiles[0].ID != "fixture" {
		t.Fatalf("persisted profile after output failure=%+v error=%v", bundle, err)
	}
}

func TestProfilesGrammarBeforeStorage(t *testing.T) {
	// Given: any attempted storage resolution would fail.
	root := isolateCLI(t)
	t.Setenv("HOME", "")
	t.Setenv("XDG_DATA_HOME", "relative")
	for _, args := range [][]string{
		{}, {"show", "--profile-index", "1"}, {"show", "--text", fixtureText},
		{"show", "--software-release", "2.0.2"}, {"show", "-"}, {"show", "PRIVATE_PATH"},
		{"show", "--json"}, {"show", "--json=true"}, {"show", "--help=true"},
		{"show", "--help", "--help"}, {"show", "--help", "PRIVATE_PATH"},
		{"show", "--"}, {"list"}, {"--json", "show"},
		{"--help"}, {" show", "--help"}, {"show ", "--help"}, {"show", "-json"},
	} {
		t.Run(strings.Join(args, "/"), func(t *testing.T) {
			reader := &observedReader{}
			var stdout, stderr bytes.Buffer
			before := cliTree(t, root)
			// When: JSON follows even invalid grammar.
			status := run(append(append([]string{"profiles"}, args...), "--json"), reader, &stdout, &stderr)
			// Then: syntax wins over storage, with the exact command envelope.
			checkCLIFailure(t, cliOutput{status, stdout.String(), stderr.String()}, 2, "profiles show", "ERR_CLI_USAGE", "usage", "null")
			if reader.reads != 0 || !reflect.DeepEqual(before, cliTree(t, root)) {
				t.Fatal("usage accessed input or changed storage")
			}
		})
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

	for _, args := range [][]string{
		{}, {"unknown"}, {"unknown", "--json"}, {"help"}, {"-h"}, {"--version"},
		{"--generate-shell-completion"}, {"version", "--help"}, {"version", "extra"},
		{"--help", "version"}, {"--help=true"}, {" version"}, {"version "},
		{" validate", "--help"}, {"profiles ", "show", "--json"},
	} {
		t.Run(strings.Join(args, "/"), func(t *testing.T) {
			reader := &observedReader{}
			var stdout, stderr bytes.Buffer
			status := run(args, reader, &stdout, &stderr)
			if status != 2 || stdout.Len() != 0 || stderr.String() != "invalid command; use --help for usage\n" || reader.reads != 0 {
				t.Fatalf("status=%d reads=%d stdout=%q stderr=%q", status, reader.reads, &stdout, &stderr)
			}
		})
	}
}

func TestRunDoesNotReadInput(t *testing.T) {
	// Given: neither help nor version may resolve even invalid storage settings.
	root := isolateCLI(t)
	t.Setenv("HOME", "")
	t.Setenv("XDG_DATA_HOME", "relative")
	for _, args := range [][]string{
		{"version"}, {"--help"}, {"validate", "--help"},
		{"validate", "--help", "--json"}, {"validate", "--json", "--help"},
		{"import", "--help"}, {"profiles", "--help"}, {"profiles", "show", "--help"},
		{"profiles", "show", "--json", "--help"}, {"profiles", "show", "--help", "--json"},
	} {
		t.Run(strings.Join(args, "/"), func(t *testing.T) {
			// Given
			reader := &observedReader{}
			var stdout, stderr bytes.Buffer
			before := cliTree(t, root)
			// When
			status := run(args, reader, &stdout, &stderr)
			// Then: help remains plain text even with --json.
			if status != 0 || reader.reads != 0 || stdout.Len() == 0 || stderr.Len() != 0 || strings.HasPrefix(stdout.String(), "{") {
				t.Fatalf("status=%d reads=%d stdout=%q stderr=%q", status, reader.reads, stdout.String(), stderr.String())
			}
			if !reflect.DeepEqual(before, cliTree(t, root)) {
				t.Fatal("help/version changed storage")
			}
		})
	}
}
