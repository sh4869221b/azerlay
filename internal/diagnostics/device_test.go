package diagnostics

import (
	"bytes"
	"github.com/sh4869221b/azerlay/internal/device"
	"strings"
	"testing"
)

func successfulRawDiscovery() (*device.Result, *device.Diagnostic) {
	serial := "PRIVATE_SERIAL"
	parent := "PRIVATE_PARENT"
	return &device.Result{Groups: []device.Group{{Complete: true}}, Nodes: []device.Node{{Path: "/dev/hidraw2", Admission: "admitted", Access: "readable", Serial: &serial, USBParent: &parent}}}, nil
}
func TestRawDeviceChecks(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*device.Result)
		code   string
		exit   int
	}{
		{"readable", func(*device.Result) {}, OK_DEVICE_ACCESS, 0},
		{"absent", func(r *device.Result) { r.Groups = nil; r.Nodes = nil }, device.ERR_DEVICE_NOT_FOUND, 1},
		{"ambiguous", func(r *device.Result) { r.Groups = append(r.Groups, r.Groups[0]) }, device.ERR_DEVICE_AMBIGUOUS, 2},
		{"permission", func(r *device.Result) { r.Nodes[0].Access = "denied" }, device.ERR_DEVICE_PERMISSION, 2},
		{"unsupported", func(r *device.Result) { r.Groups = nil; r.Nodes[0].Admission = "unsupported" }, device.ERR_DEVICE_UNSUPPORTED, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := successfulRawDiscovery()
			tc.change(r)
			calls := 0
			checks := deviceChecks(func() (*device.Result, *device.Diagnostic) { calls++; return r, nil })
			report := NewReport(checks, nil)
			if calls != 1 || report.ExitCode != tc.exit {
				t.Fatalf("%+v", report)
			}
			found := false
			for _, c := range checks {
				found = found || c.Code == tc.code
			}
			if !found {
				t.Fatal("missing raw check")
			}
			var output bytes.Buffer
			if err := WriteReport(&output, report, true); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(output.String(), "PRIVATE") {
				t.Fatal("device identity leaked")
			}
		})
	}
}
