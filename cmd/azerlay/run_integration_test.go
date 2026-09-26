package main

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/control"
)

type runFixture struct {
	env            []string
	config, socket string
}

func newRunFixture(t *testing.T) runFixture {
	t.Helper()
	root := t.TempDir()
	runtime, err := os.MkdirTemp("", "az-run-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(runtime); err != nil {
			t.Error(err)
		}
	})
	env := cliEnvironment(root)
	filtered := env[:0]
	for _, entry := range env {
		if !strings.HasPrefix(entry, "WAYLAND_DISPLAY=") && !strings.HasPrefix(entry, "XDG_RUNTIME_DIR=") {
			filtered = append(filtered, entry)
		}
	}
	env = append(filtered, "WAYLAND_DISPLAY="+os.Getenv("AZERLAY_TEST_WAYLAND_DISPLAY"), "XDG_RUNTIME_DIR="+runtime)
	config := filepath.Join(root, "XDG_CONFIG_HOME", "azerlay", "config.toml")
	if err := os.MkdirAll(filepath.Dir(config), 0700); err != nil {
		t.Fatal(err)
	}
	cliWrite(t, config, []byte("schema_version = 1\n"), 0600)
	return runFixture{env, config, filepath.Join(runtime, "azerlay", "control.sock")}
}

type runOutput struct {
	buffer bytes.Buffer
	once   sync.Once
	ready  chan struct{}
}

func (w *runOutput) Write(p []byte) (int, error) {
	n, err := w.buffer.Write(p)
	w.once.Do(func() { close(w.ready) })
	return n, err
}

type runProcess struct {
	cmd    *exec.Cmd
	stdout runOutput
	stderr bytes.Buffer
	done   chan struct{}
	err    error
}

func startRunProcess(t *testing.T, fixture runFixture, args ...string) *runProcess {
	t.Helper()
	p := &runProcess{cmd: exec.Command(cliBinary, append([]string{"run"}, args...)...), done: make(chan struct{}), stdout: runOutput{ready: make(chan struct{})}}
	p.cmd.Env = fixture.env
	p.cmd.Stdout, p.cmd.Stderr = &p.stdout, &p.stderr
	if err := p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { p.err = p.cmd.Wait(); close(p.done) }()
	t.Cleanup(func() {
		select {
		case <-p.done:
		default:
			if err := p.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				t.Error(err)
			}
			waitRunProcess(t, p)
		}
	})
	return p
}

func waitRunProcess(t *testing.T, p *runProcess) cliOutput {
	t.Helper()
	select {
	case <-p.done:
	case <-time.After(10 * time.Second):
		t.Fatal("run process did not exit")
	}
	status := 0
	if p.err != nil {
		var exit *exec.ExitError
		if !errors.As(p.err, &exit) {
			t.Fatal(p.err)
		}
		status = exit.ExitCode()
	}
	return cliOutput{status, p.stdout.buffer.String(), p.stderr.String()}
}

func readyRunProcess(t *testing.T, p *runProcess, fixture runFixture) control.Status {
	t.Helper()
	select {
	case <-p.stdout.ready:
	case <-p.done:
		t.Fatalf("run exited before readiness: %+v", waitRunProcess(t, p))
	case <-time.After(10 * time.Second):
		t.Fatal("run did not print startup status")
	}
	status := controlCLIResult[control.Status](t, fixture.env, "status")
	select {
	case <-p.done:
		t.Fatalf("run did not remain foreground: %+v", waitRunProcess(t, p))
	default:
	}
	return status
}

func stoppedRunProcess(t *testing.T, p *runProcess, fixture runFixture) {
	t.Helper()
	output := waitRunProcess(t, p)
	checkCLIStatus(t, output, 0, false)
	if output.stderr != "" {
		t.Fatalf("run stderr=%q", output.stderr)
	}
	if _, err := os.Lstat(fixture.socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket survived shutdown: %v", err)
	}
}

func TestRunForeground(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "explicit"}[explicit], func(t *testing.T) {
			fixture := newRunFixture(t)
			var args []string
			if explicit {
				args = []string{"--foreground", "--config=" + fixture.config}
			}
			p := startRunProcess(t, fixture, args...)
			status := readyRunProcess(t, p, fixture)
			if status.ActiveProfile != nil || status.LastReload.ConfigGeneration != 1 || status.RenderRate != nil || len(status.DegradedReasons) == 0 {
				t.Fatalf("unexpected no-profile status: %+v", status)
			}
			controlCLIResult[control.QuitResult](t, fixture.env, "quit")
			stoppedRunProcess(t, p, fixture)
			if !strings.Contains(p.stdout.buffer.String(), "azerlay import") || !strings.Contains(p.stdout.buffer.String(), "azerlay profiles select") {
				t.Fatal("missing profile command guidance")
			}
		})
	}
}

func TestRunLiveVisibility(t *testing.T) {
	fixture := newRunFixture(t)
	p := startRunProcess(t, fixture)
	initial := readyRunProcess(t, p, fixture)
	if initial.Visible || initial.Overlay == nil || initial.Overlay.Mapped || initial.Overlay.InputRegionApplied {
		t.Fatalf("initial status before readiness: %+v", initial)
	}
	await := func(visible bool) {
		t.Helper()
		deadline := time.NewTimer(5 * time.Second)
		defer deadline.Stop()
		for {
			status := controlCLIResult[control.Status](t, fixture.env, "status")
			if status.Visible == visible && status.Overlay != nil && status.Overlay.Mapped == visible && status.Overlay.InputRegionApplied == visible {
				return
			}
			select {
			case <-deadline.C:
				t.Fatalf("visibility did not reach requested state: %+v", status)
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
	for _, step := range []struct {
		method  string
		visible bool
	}{
		{"show", true}, {"hide", false}, {"toggle", true}, {"toggle", false},
	} {
		result := controlCLIResult[control.VisibilityResult](t, fixture.env, step.method)
		if result.Visible != step.visible {
			t.Fatalf("%s response: %+v", step.method, result)
		}
		await(step.visible)
	}
	controlCLIResult[control.QuitResult](t, fixture.env, "quit")
	stoppedRunProcess(t, p, fixture)
	if !strings.Contains(p.stdout.buffer.String(), "Overlay mapped: false") || !strings.Contains(p.stdout.buffer.String(), "Input region applied: false") {
		t.Fatalf("startup report omitted initial observation: %q", p.stdout.buffer.String())
	}
}

func TestRunDuplicate(t *testing.T) {
	fixture := newRunFixture(t)
	p := startRunProcess(t, fixture)
	readyRunProcess(t, p, fixture)
	before, err := os.Lstat(fixture.socket)
	if err != nil {
		t.Fatal(err)
	}
	output := invokeCLI(t, fixture.env, nil, "run", "--config", fixture.config+"-missing")
	checkCLIStatus(t, output, 0, false)
	if output.stdout == "" || output.stderr != "" {
		t.Fatalf("duplicate output=%+v", output)
	}
	after, err := os.Lstat(fixture.socket)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("duplicate replaced socket: %v", err)
	}
	readyRunProcess(t, p, fixture)
	controlCLIResult[control.QuitResult](t, fixture.env, "quit")
	stoppedRunProcess(t, p, fixture)
}

func TestRunConcurrent(t *testing.T) {
	fixture := newRunFixture(t)
	first, second := startRunProcess(t, fixture), startRunProcess(t, fixture)
	var owner, contender *runProcess
	select {
	case <-first.done:
		owner, contender = second, first
	case <-second.done:
		owner, contender = first, second
	case <-time.After(10 * time.Second):
		t.Fatal("both competing run processes stayed alive")
	}
	output := waitRunProcess(t, contender)
	if output.status != 0 && (output.status != 1 || !strings.Contains(output.stderr, control.ERR_CONTROL_UNAVAILABLE)) {
		t.Fatalf("unexpected contender result: %+v", output)
	}
	readyRunProcess(t, owner, fixture)
	checkCLIStatus(t, invokeCLI(t, fixture.env, nil, "run"), 0, false)
	controlCLIResult[control.QuitResult](t, fixture.env, "quit")
	stoppedRunProcess(t, owner, fixture)
}

func TestRunStale(t *testing.T) {
	fixture := newRunFixture(t)
	if err := os.Mkdir(filepath.Dir(fixture.socket), 0700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: fixture.socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	listener.SetUnlinkOnClose(false)
	if err := os.Chmod(fixture.socket, 0600); err != nil {
		t.Fatal(err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	p := startRunProcess(t, fixture)
	readyRunProcess(t, p, fixture)
	controlCLIResult[control.QuitResult](t, fixture.env, "quit")
	stoppedRunProcess(t, p, fixture)
}

func TestRunShutdown(t *testing.T) {
	for _, signal := range []os.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(signal.String(), func(t *testing.T) {
			fixture := newRunFixture(t)
			p := startRunProcess(t, fixture, "--config", fixture.config)
			readyRunProcess(t, p, fixture)
			if err := p.cmd.Process.Signal(signal); err != nil {
				t.Fatal(err)
			}
			stoppedRunProcess(t, p, fixture)
			restarted := startRunProcess(t, fixture)
			readyRunProcess(t, restarted, fixture)
			controlCLIResult[control.QuitResult](t, fixture.env, "quit")
			stoppedRunProcess(t, restarted, fixture)
		})
	}
}

func TestRunBrokenPipe(t *testing.T) {
	fixture := newRunFixture(t)
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
	cmd := exec.CommandContext(ctx, cliBinary, "run")
	cmd.Env, cmd.Stdout = fixture.env, writer
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	var exit *exec.ExitError
	if ctx.Err() != nil || !errors.As(err, &exit) || exit.ExitCode() != 1 || stderr.Len() != 0 {
		t.Fatalf("broken pipe exit=%v context=%v stderr=%q", err, ctx.Err(), &stderr)
	}
	if _, err := os.Lstat(fixture.socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("broken pipe left socket: %v", err)
	}
	restarted := startRunProcess(t, fixture)
	readyRunProcess(t, restarted, fixture)
	controlCLIResult[control.QuitResult](t, fixture.env, "quit")
	stoppedRunProcess(t, restarted, fixture)
}
