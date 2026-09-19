package main

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/control"
)

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
