package main

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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

func TestRunLocalWithoutImportedHome(t *testing.T) {
	for _, policy := range []string{"local", "auto"} {
		t.Run(policy, func(t *testing.T) {
			fixture := newRunFixture(t)
			store := filepath.Join(doctorRoot(fixture), "userData")
			path := filepath.Join(store, "Storage/DevicesStorage/device/ProfileStorage/profile_a.json")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			cliWrite(t, path, []byte(`{"id":"a","name":"Synthetic local","version":1,"inputs":[],"isSoftware":true,"metaData":{"changedLogs":[{"softwareVersion":"2.0.2"}]}}`), 0600)
			cliWrite(t, fixture.config, []byte("schema_version=1\n[profile]\nsource='"+policy+"'\nlocal_store_path='"+store+"'\nlocal_device='device'\nlocal_profile_file='profile_a.json'\n"), 0600)
			for index, entry := range fixture.env {
				if strings.HasPrefix(entry, "HOME=") {
					fixture.env[index] = "HOME="
				} else if strings.HasPrefix(entry, "XDG_DATA_HOME=") {
					fixture.env[index] = "XDG_DATA_HOME=relative"
				}
			}
			process := startRunProcess(t, fixture, "--config", fixture.config)
			status := readyRunProcess(t, process, fixture)
			if status.ActiveProfile == nil || status.ActiveProfile.Source != "local" || *status.ActiveProfile.Name != "Synthetic local" {
				t.Fatalf("import placement blocked explicit local: %+v", status)
			}
			controlCLIResult[control.QuitResult](t, fixture.env, "quit")
			stoppedRunProcess(t, process, fixture)
		})
	}
}

func TestRunLocalLifecycle(t *testing.T) {
	fixture := newRunFixture(t)
	store := filepath.Join(doctorRoot(fixture), "userData")
	ref := "Storage/DevicesStorage/device/ProfileStorage/profile_a.json"
	path := filepath.Join(store, ref)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	data := `{"id":"a","name":"Initial local","version":1,"inputs":[],"isSoftware":true,"metaData":{"changedLogs":[{"softwareVersion":"2.0.2"}]}}`
	cliWrite(t, path, []byte(data), 0600)
	cliWrite(t, fixture.config, []byte("schema_version=1\n[profile]\nsource='local'\nlocal_store_path='"+store+"'\nlocal_device='device'\nlocal_profile_file='profile_a.json'\n"), 0600)
	process := startRunProcess(t, fixture)
	initial := readyRunProcess(t, process, fixture)
	if initial.ActiveProfile == nil || initial.ActiveProfile.Source != "local" || initial.ActiveProfile.SourceRef != ref || initial.ActiveProfile.ProfileIndex != 1 || *initial.ActiveProfile.Name != "Initial local" {
		t.Fatalf("local startup: %+v", initial)
	}
	await := func(name, code string) control.Status {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			status := controlCLIResult[control.Status](t, fixture.env, "status")
			failure := ""
			for _, diagnostic := range status.DegradedReasons {
				if strings.HasPrefix(diagnostic.Code, "ERR_PROFILE_LOCAL_") {
					failure = diagnostic.Code
					if strings.Contains(diagnostic.Reason, store) || strings.Contains(diagnostic.Reason, "PRIVATE_") {
						t.Fatalf("private local failure: %+v", diagnostic)
					}
				}
			}
			if status.ActiveProfile != nil && status.ActiveProfile.Source == "local" && status.ActiveProfile.SourceRef == ref && status.ActiveProfile.ProfileIndex == 1 && *status.ActiveProfile.Name == name && failure == code {
				return status
			}
			if time.Now().After(deadline) {
				t.Fatalf("local transition name=%q code=%q: %+v", name, code, status)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	cliWrite(t, path, []byte(`{"PRIVATE_PARTIAL":`), 0600)
	failed := await("Initial local", "ERR_PROFILE_LOCAL_UNSUPPORTED")
	if failed.Generation < initial.Generation || failed.ActiveProfile.ProfileIndex != initial.ActiveProfile.ProfileIndex || failed.ActiveProfile.SourceRef != initial.ActiveProfile.SourceRef {
		t.Fatal("partial write replaced the last good profile or regressed runtime generation")
	}
	repaired := strings.Replace(data, "Initial local", "Repaired local", 1)
	temporary := filepath.Join(filepath.Dir(path), "writer.tmp")
	cliWrite(t, temporary, []byte(repaired), 0600)
	if err := os.Rename(temporary, path); err != nil {
		t.Fatal(err)
	}
	updated := await("Repaired local", "")
	if updated.Generation <= failed.Generation {
		t.Fatal("rename repair did not change generation")
	}
	cliWrite(t, path, []byte(strings.Replace(repaired, "2.0.2", "9.9.9", 1)), 0600)
	await("Repaired local", "ERR_PROFILE_LOCAL_UNSUPPORTED")
	controlCLIResult[control.QuitResult](t, fixture.env, "quit")
	stoppedRunProcess(t, process, fixture)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	before := cliTree(t, store)
	restarted := startRunProcess(t, fixture)
	readyRunProcess(t, restarted, fixture)
	await("Repaired local", "ERR_PROFILE_LOCAL_READ")
	controlCLIResult[control.QuitResult](t, fixture.env, "quit")
	stoppedRunProcess(t, restarted, fixture)
	if current := cliTree(t, store); !reflect.DeepEqual(before, current) {
		t.Fatal("restart wrote into source store")
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
