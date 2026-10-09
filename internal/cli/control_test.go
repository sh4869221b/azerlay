package cli

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

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

func TestControlLiveStatusReport(t *testing.T) {
	t.Parallel()
	device, events, draws, dropped := "usb-synthetic-device", 42.5, 60.0, uint64(3)
	text := renderControlReport(control.Status{Device: &device, EventNodes: []string{}, EventRate: &events, RenderRate: &draws, DroppedCount: &dropped})
	for _, line := range []string{"Device: usb-synthetic-device", "Event nodes: []", "Event rate: 42.500", "Render rate: 60.000", "Dropped count: 3", "Resync count: unavailable"} {
		if !strings.Contains(text, line) {
			t.Fatalf("missing %q: %s", line, text)
		}
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
