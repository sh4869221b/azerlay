package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCLIDevicesHelp(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"devices", "--help"},
		{"devices", "list", "--help"},
		{"devices", "list", "--json", "--help"},
		{"devices", "inspect", "--help"},
		{"devices", "inspect", "--json", "--help"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			command := "devices"
			if args[1] != "--help" {
				command += " " + args[1]
			}
			output := invokeCLI(t, cliEnvironment(t.TempDir()), nil, args...)
			checkCLIStatus(t, output, 0, true)
			if !strings.HasPrefix(output.stdout, "Usage: azerlay "+command+" ") {
				t.Fatalf("help=%q", output.stdout)
			}
		})
	}
}

func TestCLIDevicesUsage(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		args    []string
		command string
	}{
		{"missing subcommand", []string{"devices"}, "devices"},
		{"flag before subcommand", []string{"devices", "--help", "list"}, "devices"},
		{"unknown flag", []string{"devices", "list", "--unknown"}, "devices list"},
		{"list positional", []string{"devices", "list", "event0"}, "devices list"},
		{"extra inspect path", []string{"devices", "inspect", "event0", "event1"}, "devices inspect"},
		{"boolean equals", []string{"devices", "list", "--json=true"}, "devices list"},
		{"duplicate flag", []string{"devices", "list", "--help", "--help"}, "devices list"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := cliEnvironment(t.TempDir())
			t.Run("text", func(t *testing.T) {
				output := invokeCLI(t, env, nil, tc.args...)
				checkCLIStatus(t, output, 2, false)
				if output.stdout != "" || !strings.Contains(output.stderr, "ERR_CLI_USAGE") {
					t.Fatalf("usage stdout=%q stderr=%q", output.stdout, output.stderr)
				}
			})
			t.Run("JSON", func(t *testing.T) {
				output := invokeCLI(t, env, nil, append(tc.args, "--json")...)
				checkCLIStatus(t, output, 2, true)
				parsedJSON(t, output.stdout)
				var report devicesReport
				if err := json.Unmarshal([]byte(output.stdout), &report); err != nil {
					t.Fatal(err)
				}
				if report.SchemaVersion != 2 || report.Command != tc.command || report.OK || report.Result != nil || report.Error == nil || report.Error.Code != "ERR_CLI_USAGE" || report.Error.Stage != "usage" {
					t.Fatalf("usage report=%+v", report)
				}
			})
		})
	}
}

func TestCLIDevicesInspectMissingPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "event0")
	output := invokeCLI(t, cliEnvironment(root), nil, "devices", "inspect", path, "--json")
	checkCLIStatus(t, output, 1, true)
	parsedJSON(t, output.stdout)
	var report devicesReport
	if err := json.Unmarshal([]byte(output.stdout), &report); err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != 2 || report.Command != "devices inspect" || report.OK || report.Result != nil || report.Error == nil || report.Error.Code != "ERR_DEVICE_NOT_FOUND" || report.Error.Stage != "resolution" || report.Error.Target == nil || *report.Error.Target != path {
		t.Fatalf("missing path report=%+v", report)
	}
}

func TestCLIDevicesBrokenPipe(t *testing.T) {
	t.Parallel()
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
	cmd := exec.CommandContext(ctx, cliBinary, "devices", "list", "--help")
	cmd.Env, cmd.Stdout = cliEnvironment(t.TempDir()), writer
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	var exit *exec.ExitError
	if ctx.Err() != nil || !errors.As(err, &exit) || exit.ExitCode() != 1 || stderr.Len() != 0 {
		t.Fatalf("broken pipe exit=%v context=%v stderr=%q", err, ctx.Err(), stderr.String())
	}
}
