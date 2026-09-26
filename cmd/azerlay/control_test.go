package main

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/control"
)

func TestControlOverlayStatusReport(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		overlay *control.OverlayStatus
		mapped  string
		region  string
	}{
		{"unattached", nil, "", ""},
		{"hidden", &control.OverlayStatus{}, "Overlay mapped: false", "Input region applied: false"},
		{"mapped", &control.OverlayStatus{Mapped: true, InputRegionApplied: true}, "Overlay mapped: true", "Input region applied: true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status := control.Status{SchemaVersion: 1, Visible: true, Overlay: tc.overlay, EventNodes: []string{}, Generation: 2, DegradedReasons: []control.Diagnostic{{Code: "ERR_OVERLAY_INPUT_REGION", Stage: "overlay", Reason: "Input region is unavailable."}}}
			report := controlReport{SchemaVersion: 1, Command: "status", OK: true, Result: status}
			var stdout, stderr bytes.Buffer
			if code := writeControlReport(report, false, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
				t.Fatalf("text code=%d stderr=%q", code, &stderr)
			}
			if !strings.Contains(stdout.String(), "Requested visibility: true") || !strings.Contains(stdout.String(), "Degraded: ERR_OVERLAY_INPUT_REGION (overlay): Input region is unavailable.") {
				t.Fatalf("text contract: %q", &stdout)
			}
			for _, line := range []string{tc.mapped, tc.region} {
				if line != "" && !strings.Contains(stdout.String(), line) {
					t.Fatalf("missing %q in %q", line, &stdout)
				}
			}
			if tc.overlay == nil && strings.Contains(stdout.String(), "Overlay mapped:") {
				t.Fatalf("unattached state appeared: %q", &stdout)
			}
			stdout.Reset()
			if code := writeControlReport(report, true, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
				t.Fatalf("JSON code=%d stderr=%q", code, &stderr)
			}
			var decoded struct {
				Result struct {
					Visible         bool                       `json:"visible"`
					Overlay         map[string]json.RawMessage `json:"overlay"`
					DegradedReasons []control.Diagnostic       `json:"degraded_reasons"`
				} `json:"result"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
				t.Fatal(err)
			}
			if !decoded.Result.Visible || decoded.Result.DegradedReasons[0].Stage != "overlay" {
				t.Fatalf("JSON status: %+v", decoded.Result)
			}
			if tc.overlay == nil {
				if decoded.Result.Overlay != nil {
					t.Fatalf("unattached JSON overlay: %+v", decoded.Result.Overlay)
				}
				return
			}
			if len(decoded.Result.Overlay) != 2 || string(decoded.Result.Overlay["mapped"]) != strconv.FormatBool(tc.overlay.Mapped) || string(decoded.Result.Overlay["input_region_applied"]) != strconv.FormatBool(tc.overlay.InputRegionApplied) {
				t.Fatalf("JSON overlay types and values: %+v", decoded.Result.Overlay)
			}
		})
	}
}

func TestControlGrammar(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "PRIVATE_INVALID_RUNTIME")
	for _, command := range [][]string{{"show"}, {"hide"}, {"toggle"}, {"reload"}, {"status"}, {"quit"}, {"profiles", "select"}} {
		t.Run(strings.Join(command, " "), func(t *testing.T) {
			for _, suffix := range [][]string{{"--json", "--help"}, {"--help", "--json"}} {
				var stdout, stderr bytes.Buffer
				reader := &observedReader{}
				status := run(append(append([]string{}, command...), suffix...), reader, &stdout, &stderr)
				if status != 0 || stderr.Len() != 0 || stdout.Len() == 0 || reader.reads != 0 {
					t.Fatalf("help status=%d stdout=%q stderr=%q reads=%d", status, &stdout, &stderr, reader.reads)
				}
			}
			for _, suffix := range [][]string{{"--wat"}, {"--help", "--help"}, {"--json=true"}, {"--help=true"}, {"--text", "PRIVATE_INPUT"}, {"one", "two"}, {""}} {
				var stdout, stderr bytes.Buffer
				status := run(append(append(append([]string{}, command...), "--json"), suffix...), &observedReader{}, &stdout, &stderr)
				checkCLIFailure(t, cliOutput{status, stdout.String(), stderr.String()}, 2, strings.Join(command, " "), "ERR_CLI_USAGE", "usage", "null")
			}
		})
	}
	var stdout, stderr bytes.Buffer
	status := run([]string{"profiles", "select", "--json"}, &observedReader{}, &stdout, &stderr)
	checkCLIFailure(t, cliOutput{status, stdout.String(), stderr.String()}, 2, "profiles select", "ERR_CLI_USAGE", "usage", "null")
}

func TestControlOutputFailures(t *testing.T) {
	t.Parallel()
	for _, jsonMode := range []bool{false, true} {
		writer := &refusingWriter{}
		var stderr bytes.Buffer
		status := writeControlReport(controlReport{SchemaVersion: 1, Command: "show", OK: true, Result: control.VisibilityResult{Visible: true}}, jsonMode, writer, &stderr)
		if status != 1 || writer.calls != 1 || stderr.Len() != 0 {
			t.Fatalf("status=%d writes=%d stderr=%q", status, writer.calls, &stderr)
		}
	}
}

func TestCLIControlMalformedResponse(t *testing.T) {
	// Given: a real socket peer that sends a malformed response only after a status request.
	runtime, err := os.MkdirTemp("", "az-cli-bad-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(runtime); err != nil {
			t.Error(err)
		}
	})
	path := filepath.Join(runtime, "azerlay")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(path, "control.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	env := cliEnvironment(t.TempDir())
	for i, entry := range env {
		if strings.HasPrefix(entry, "XDG_RUNTIME_DIR=") {
			env[i] = "XDG_RUNTIME_DIR=" + runtime
		}
	}
	// When: help and invalid syntax are evaluated against a live listening socket.
	for _, args := range [][]string{{"show", "--help"}, {"profiles", "select", "--help"}, {"hide", "--json", "PRIVATE_EXTRA"}, {"profiles", "select", "--json"}} {
		output := invokeCLI(t, env, nil, args...)
		want := 2
		if args[len(args)-1] == "--help" {
			want = 0
		}
		checkCLIStatus(t, output, want, true)
	}
	if err := listener.SetDeadline(time.Now()); err != nil {
		t.Fatal(err)
	}
	if connection, err := listener.AcceptUnix(); err == nil {
		connection.Close()
		t.Fatal("help or invalid syntax connected")
	} else if failure, ok := err.(net.Error); !ok || !failure.Timeout() {
		t.Fatal(err)
	}
	if err := listener.SetDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		connection, err := listener.AcceptUnix()
		if err != nil {
			done <- err
			return
		}
		defer connection.Close()
		if err := connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			done <- err
			return
		}
		if _, err := control.ReadRequest(connection); err != nil {
			done <- err
			return
		}
		_, err = connection.Write([]byte("{PRIVATE_MALFORMED_RESPONSE}\n"))
		done <- err
	}()
	// Then: the built client reports a stable error without echoing the response.
	output := invokeCLI(t, env, nil, "status", "--json")
	checkCLIFailure(t, output, 1, "status", "ERR_CONTROL_REQUEST", "protocol", "null")
	if strings.Contains(output.stdout, "PRIVATE_MALFORMED_RESPONSE") {
		t.Fatal("malformed response was disclosed")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("malformed peer did not finish")
	}
}
