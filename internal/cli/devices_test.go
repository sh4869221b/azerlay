package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/sh4869221b/azerlay/internal/device"
)

func TestDevicesHelpWithoutCollection(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"--help"}, {"list", "--help", "--json"}, {"inspect", "literal-path", "--help", "--json"}} {
		_, o, valid := normalizeDevicesArgs(args)
		if !valid {
			t.Fatalf("valid help rejected: %v", args)
		}
		var out, errOut bytes.Buffer
		collect := func(string, string) (*device.Result, *device.Diagnostic) {
			t.Fatal("help collected devices")
			return nil, nil
		}
		if status := runDevices(o, collect, &out, &errOut); status != 0 || !strings.Contains(out.String(), "Usage: azerlay devices") || errOut.Len() != 0 {
			t.Fatalf("status=%d out=%q err=%q", status, &out, &errOut)
		}
		out.Reset()
		if status := run(append([]string{"devices"}, args...), &observedReader{}, &out, &errOut); status != 0 || !strings.Contains(out.String(), "Usage: azerlay devices") || errOut.Len() != 0 {
			t.Fatalf("registered help: status=%d out=%q err=%q", status, &out, &errOut)
		}
	}
}

func TestDevicesStrictGrammar(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args    []string
		command string
		json    bool
	}{
		{nil, "devices", false},
		{[]string{"unknown", "--json"}, "devices", true},
		{[]string{"--json", "list"}, "devices", true},
		{[]string{"--help", "--json"}, "devices", true},
		{[]string{"list", "path", "--help", "--json"}, "devices list", true},
		{[]string{"inspect", "one", "two", "--json"}, "devices inspect", true},
		{[]string{"list", "--json=true"}, "devices list", false},
		{[]string{"list", "--help=true", "--json"}, "devices list", true},
		{[]string{"list", "--json", "--json"}, "devices list", true},
		{[]string{"list", "--help", "--help", "--json"}, "devices list", true},
		{[]string{"list", "--unknown", "--json"}, "devices list", true},
		{[]string{"list", "--", "--json"}, "devices list", false},
		{[]string{"inspect", "--", "one", "two", "--json"}, "devices inspect", false},
		{[]string{"inspect", ""}, "devices inspect", false},
	} {
		t.Run(strings.Join(tc.args, "/"), func(t *testing.T) {
			_, o, valid := normalizeDevicesArgs(tc.args)
			if valid || o.command != tc.command || o.json != tc.json {
				t.Fatalf("normalizer: %+v valid=%t", o, valid)
			}
			var out, errOut bytes.Buffer
			reader := &observedReader{}
			status := run(append([]string{"devices"}, tc.args...), reader, &out, &errOut)
			if status != 2 || reader.reads != 0 {
				t.Fatalf("status=%d reads=%d", status, reader.reads)
			}
			if tc.json {
				var r devicesReport
				if err := json.Unmarshal(out.Bytes(), &r); err != nil {
					t.Fatal(err)
				}
				if errOut.Len() != 0 || r.SchemaVersion != 2 || r.OK || r.Command != tc.command || r.Result != nil || r.Error == nil || r.Error.Code != "ERR_CLI_USAGE" {
					t.Fatalf("usage report: %+v stderr=%q", r, &errOut)
				}
			} else if out.Len() != 0 || !strings.Contains(errOut.String(), "ERR_CLI_USAGE") {
				t.Fatalf("text usage out=%q err=%q", &out, &errOut)
			}
		})
	}
}

func TestDevicesExplicitLocator(t *testing.T) {
	t.Parallel()
	args := []string{"inspect", "--json", "--", "--literal path\n"}
	normalized, o, valid := normalizeDevicesArgs(args)
	if !valid || !reflect.DeepEqual(normalized, args) || o.path != "--literal path\n" {
		t.Fatalf("literal changed: %q %+v", normalized, o)
	}
	calls := 0
	collect := func(command, path string) (*device.Result, *device.Diagnostic) {
		calls++
		if command != "devices inspect" || path != o.path {
			t.Fatalf("collection target=%q/%q", command, path)
		}
		return nil, &device.Diagnostic{Code: device.ERR_DEVICE_NOT_FOUND, Severity: "error", Stage: "resolution", Target: &path}
	}
	var out, errOut bytes.Buffer
	if status := runDevices(o, collect, &out, &errOut); status != 1 || calls != 1 || errOut.Len() != 0 {
		t.Fatalf("status=%d calls=%d stderr=%q", status, calls, &errOut)
	}
	var r devicesReport
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Result != nil || r.Error == nil || *r.Error.Target != o.path {
		t.Fatalf("fatal report: %+v", r)
	}
	_, literal, valid := normalizeDevicesArgs([]string{"inspect", "--", "--json"})
	if !valid || literal.json || literal.path != "--json" {
		t.Fatal("literal --json became a flag")
	}
}

func TestDevicesOutputFailure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		options devicesOptions
		stderr  bool
	}{
		{"help", devicesOptions{command: "devices list", help: true}, false},
		{"json", devicesOptions{command: "devices list", json: true}, false},
		{"text error", devicesOptions{command: "devices inspect"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writer := &refusingWriter{}
			var other bytes.Buffer
			var out, errOut io.Writer = writer, &other
			if tc.stderr {
				out, errOut = &other, writer
			}
			collect := func(string, string) (*device.Result, *device.Diagnostic) {
				return nil, &device.Diagnostic{Code: device.ERR_DEVICE_METADATA, Severity: "error", Stage: "metadata"}
			}
			if status := runDevices(tc.options, collect, out, errOut); status != 1 || writer.calls != 1 || other.Len() != 0 {
				t.Fatalf("status=%d calls=%d other=%q", status, writer.calls, &other)
			}
		})
	}
}
