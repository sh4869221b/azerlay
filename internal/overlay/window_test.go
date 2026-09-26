package overlay

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

const nativeChildEnv = "AZERLAY_OVERLAY_NATIVE_CHILD"

func TestMain(m *testing.M) {
	if mode := os.Getenv(nativeChildEnv); mode != "" {
		runtime.LockOSThread()
		os.Exit(runNativeChild(mode))
	}
	os.Exit(m.Run())
}

func TestNativeWindow(t *testing.T) {
	if os.Getenv("AZERLAY_TEST_WAYLAND_DISPLAY") == "" {
		launcher, err := filepath.Abs("../../scripts/test-wayland.sh")
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(t.Context(), "bash", launcher, os.Args[0], "-test.run=^TestNativeWindow$", "-test.v")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Wayland fixture: %v\n%s", err, output)
		} else {
			t.Log(string(output))
		}
		return
	}
	for _, mode := range []string{"lifecycle", "region-failure", "invalid-display"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0])
			cmd.Env = append(os.Environ(), nativeChildEnv+"="+mode, "GDK_BACKEND=wayland", "WAYLAND_DISPLAY="+os.Getenv("AZERLAY_TEST_WAYLAND_DISPLAY"))
			if mode == "invalid-display" {
				cmd.Env = append(cmd.Env, "WAYLAND_DISPLAY=azerlay-missing-display", "DISPLAY=:azerlay-invalid")
			}
			stdin, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			waited := false
			wait := func() error {
				if waited {
					return nil
				}
				waited = true
				return cmd.Wait()
			}
			defer func() { cancel(); wait() }()
			scanner := bufio.NewScanner(stdout)
			want := []string{"ready", "hidden", "missing", "restored", "explicit", "default-return", "hidden-return", "closed"}
			if mode == "invalid-display" {
				want = []string{"display-unavailable"}
			} else if mode == "region-failure" {
				want = []string{"failed", "recovered", "closed"}
			}
			for i, expected := range want {
				if !scanner.Scan() {
					cancel()
					wait()
					t.Fatalf("child ended before %q: scan=%v stderr=%s", expected, scanner.Err(), stderr.String())
				}
				line := scanner.Text()
				t.Log(line)
				if line != expected {
					cancel()
					wait()
					t.Fatalf("child state = %q; want %q; stderr=%s", line, expected, stderr.String())
				}
				if mode != "invalid-display" && i < len(want)-1 {
					command := "continue"
					if i == len(want)-2 {
						command = "quit"
					}
					if _, err := fmt.Fprintln(stdin, command); err != nil {
						t.Fatal(err)
					}
				}
			}
			stdin.Close()
			if err := wait(); err != nil {
				t.Fatalf("child failed: %v; stderr=%s", err, stderr.String())
			}
			if ctx.Err() != nil {
				t.Fatalf("child timeout: %v", ctx.Err())
			}
		})
	}
}
