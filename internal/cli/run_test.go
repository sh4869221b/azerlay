package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunUsage(t *testing.T) {
	root := t.TempDir()
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "runtime"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	for _, tc := range []struct {
		name   string
		args   []string
		status int
	}{
		{"help", []string{"--help"}, 0},
		{"help_with_config", []string{"--config", "missing", "--foreground", "--help"}, 0},
		{"empty_config", []string{"--config="}, 2},
		{"empty_separate_config", []string{"--config", ""}, 2},
		{"missing_config_value", []string{"--config"}, 2},
		{"duplicate_config", []string{"--config=a", "--config=b"}, 2},
		{"duplicate_boolean", []string{"--foreground", "--foreground"}, 2},
		{"boolean_value", []string{"--foreground=true"}, 2},
		{"help_value", []string{"--help=false"}, 2},
		{"unknown", []string{"--json"}, 2},
		{"positional", []string{"path"}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			status := run(append([]string{"run"}, tc.args...), nil, &stdout, &stderr)
			if status != tc.status {
				t.Fatalf("status=%d want=%d stderr=%q", status, tc.status, &stderr)
			}
			if tc.status == 2 && (!strings.Contains(stderr.String(), "ERR_CLI_USAGE (usage)") || stdout.Len() != 0) {
				t.Fatalf("invalid usage output: %q / %q", &stdout, &stderr)
			}
		})
	}
	if status := run([]string{"run", "--help"}, nil, &refusingWriter{}, io.Discard); status != 1 {
		t.Fatalf("help output failure status=%d", status)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("help/usage created resources: %v %v", entries, err)
	}
}
