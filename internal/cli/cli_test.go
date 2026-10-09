package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestRunForegroundDispatch(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		args       []string
		configPath string
	}{
		{"default", []string{"run"}, ""},
		{"foreground", []string{"run", "--foreground"}, ""},
		{"separate config", []string{"run", "--config", " config path "}, " config path "},
		{"equals config", []string{"run", "--foreground", "--config= config path "}, " config path "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &observedReader{}
			var stdout, stderr bytes.Buffer
			calls := 0
			status := Run(tc.args, "test-version", reader, &stdout, &stderr, func(configPath string, out, errOut io.Writer) int {
				calls++
				if configPath != tc.configPath || out != &stdout || errOut != &stderr {
					t.Fatalf("native dispatch config=%q stdout=%T stderr=%T", configPath, out, errOut)
				}
				// A distinctive status verifies forwarding, not native startup.
				return 37
			})
			if status != 37 || calls != 1 || reader.reads != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("status=%d calls=%d reads=%d stdout=%q stderr=%q", status, calls, reader.reads, &stdout, &stderr)
			}
		})
	}
}

func TestRunVersionValue(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	status := Run([]string{"version"}, "test-version", strings.NewReader(""), &stdout, &stderr, func(string, io.Writer, io.Writer) int {
		t.Fatal("version entered native startup")
		return 1
	})
	if status != 0 || stdout.String() != "azerlay test-version\n" || stderr.Len() != 0 {
		t.Fatalf("version status=%d stdout=%q stderr=%q", status, &stdout, &stderr)
	}
}
