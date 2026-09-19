package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/control"
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

func TestRunNoWayland(t *testing.T) {
	root := t.TempDir()
	env := cliEnvironment(root)
	for i, entry := range env {
		if strings.HasPrefix(entry, "WAYLAND_DISPLAY=") {
			env[i] = "WAYLAND_DISPLAY="
		}
	}
	env = append(env, "WAYLAND_DISPLAY=")
	output := invokeCLI(t, env, nil, "run")
	checkCLIStatus(t, output, 1, false)
	if output.stdout != "" || !strings.Contains(output.stderr, "ERR_RUNTIME_WAYLAND (runtime)") {
		t.Fatalf("unexpected admission error: %+v", output)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("Wayland rejection created resources: %v %v", entries, err)
	}
}

func TestRunConfigFailure(t *testing.T) {
	for _, tc := range []struct{ name, contents, code, stage string }{
		{"missing", "", "ERR_CONFIG_NOT_FOUND", "read"},
		{"invalid", "PRIVATE_CONFIG = [\n", "ERR_CONFIG_INVALID", "decode"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newRunFixture(t)
			if tc.contents == "" {
				if err := os.Remove(fixture.config); err != nil {
					t.Fatal(err)
				}
			} else {
				cliWrite(t, fixture.config, []byte(tc.contents), 0600)
			}
			output := invokeCLI(t, fixture.env, nil, "run")
			checkCLIStatus(t, output, 1, false)
			if output.stdout != "" || !strings.Contains(output.stderr, tc.code+" ("+tc.stage+")") || strings.Contains(output.stderr, fixture.config) || strings.Contains(output.stderr, "PRIVATE_CONFIG") {
				t.Fatalf("unsafe or incorrect config failure: %+v", output)
			}
			if _, err := os.Lstat(fixture.socket); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed startup left socket: %v", err)
			}
			cliWrite(t, fixture.config, []byte("schema_version = 1\n"), 0600)
			p := startRunProcess(t, fixture)
			readyRunProcess(t, p, fixture)
			controlCLIResult[control.QuitResult](t, fixture.env, "quit")
			stoppedRunProcess(t, p, fixture)
		})
	}
}

func setRunEnvironment(t *testing.T, fixture runFixture) {
	t.Helper()
	for _, entry := range fixture.env {
		key, value, _ := strings.Cut(entry, "=")
		if key == "HOME" || key == "WAYLAND_DISPLAY" || strings.HasPrefix(key, "XDG_") {
			t.Setenv(key, value)
		}
	}
}

func checkRunOwnerReleased(t *testing.T, fixture runFixture) {
	t.Helper()
	if _, err := os.Lstat(fixture.socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket survived: %v", err)
	}
	owner, status, err := control.AcquireInstance(t.Context())
	if err != nil || owner == nil || status != nil {
		t.Fatalf("owner not released: owner=%v status=%v err=%v", owner, status, err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRunStartupCleanup(t *testing.T) {
	t.Run("output_failure", func(t *testing.T) {
		fixture := newRunFixture(t)
		setRunEnvironment(t, fixture)
		var stderr bytes.Buffer
		status := run([]string{"run"}, nil, &refusingWriter{}, &stderr)
		if status != 1 || stderr.Len() != 0 {
			t.Fatalf("output failure=%d stderr=%q", status, &stderr)
		}
		checkRunOwnerReleased(t, fixture)
	})
	t.Run("data_home_failure", func(t *testing.T) {
		fixture := newRunFixture(t)
		setRunEnvironment(t, fixture)
		t.Setenv("HOME", "")
		t.Setenv("XDG_DATA_HOME", "")
		var stderr bytes.Buffer
		status := runForeground(t.Context(), fixture.config, io.Discard, &stderr)
		if status != 1 || !strings.Contains(stderr.String(), "ERR_PROFILE_STORAGE (storage)") {
			t.Fatalf("storage error=%d stderr=%q", status, &stderr)
		}
		checkRunOwnerReleased(t, fixture)
	})
	t.Run("unsafe_occupied_path", func(t *testing.T) {
		fixture := newRunFixture(t)
		if err := os.Mkdir(filepath.Dir(fixture.socket), 0700); err != nil {
			t.Fatal(err)
		}
		cliWrite(t, fixture.socket, []byte("preserve"), 0600)
		output := invokeCLI(t, fixture.env, nil, "run")
		checkCLIStatus(t, output, 1, false)
		if !strings.Contains(output.stderr, control.ERR_CONTROL_PERMISSION) || string(cliRead(t, fixture.socket)) != "preserve" {
			t.Fatalf("unsafe path not preserved: %+v", output)
		}
	})
	for _, cancelled := range []bool{true, false} {
		t.Run(map[bool]string{true: "cancel_during_config", false: "server_start_failure"}[cancelled], func(t *testing.T) {
			fixture := newRunFixture(t)
			setRunEnvironment(t, fixture)
			fifo := filepath.Join(filepath.Dir(fixture.config), "startup-config")
			if err := syscall.Mkfifo(fifo, 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			result := make(chan int, 1)
			var stderr bytes.Buffer
			go func() { result <- runForeground(ctx, fifo, io.Discard, &stderr) }()
			writerReady := make(chan *os.File, 1)
			openError := make(chan error, 1)
			go func() {
				writer, err := os.OpenFile(fifo, os.O_WRONLY, 0)
				if err != nil {
					openError <- err
				} else {
					writerReady <- writer
				}
			}()
			var writer *os.File
			select {
			case writer = <-writerReady:
			case err := <-openError:
				t.Fatal(err)
			case <-time.After(5 * time.Second):
				t.Fatal("config reader did not open")
			}
			t.Cleanup(func() { writer.Close() })
			if cancelled {
				cancel()
				select {
				case status := <-result:
					t.Fatalf("startup returned %d before joining blocked config read", status)
				default:
				}
			} else {
				cliWrite(t, fixture.socket, []byte("preserve"), 0600)
			}
			if _, err := io.WriteString(writer, "schema_version = 1\n"); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case status := <-result:
				want := 1
				if cancelled {
					want = 0
				}
				if status != want {
					t.Fatalf("startup status=%d want=%d stderr=%q", status, want, &stderr)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("startup did not join config")
			}
			if !cancelled {
				if string(cliRead(t, fixture.socket)) != "preserve" {
					t.Fatal("server failure removed occupied path")
				}
				if err := os.Remove(fixture.socket); err != nil {
					t.Fatal(err)
				}
			}
			checkRunOwnerReleased(t, fixture)
		})
	}
}
