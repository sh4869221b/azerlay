package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sh4869221b/azerlay/internal/diagnostics"
)

func TestDoctorUsage(t *testing.T) {
	setDoctorEnvironment(t)
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
	setDoctorEnvironment(t)
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

// Use only isolated filesystem/configuration state. No compositor, native
// fixture, socket peer or executable is needed to check command/output behavior.
func setDoctorEnvironment(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	for _, key := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "XDG_RUNTIME_DIR"} {
		t.Setenv(key, filepath.Join(root, key))
	}
	t.Setenv("WAYLAND_DISPLAY", "")
	configPath := filepath.Join(root, "XDG_CONFIG_HOME", "azerlay", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		t.Fatal(err)
	}
	cliWrite(t, configPath, []byte("schema_version = 1\n"), 0600)
}
