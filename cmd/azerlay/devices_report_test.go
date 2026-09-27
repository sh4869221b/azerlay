package main

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/sh4869221b/azerlay/internal/device"
)

func devicesFixture() *device.Result {
	parent, name, serial, physical, sysfs := "/synthetic/usb-a", "Synthetic\nName\x1b", "synthetic-secret", "synthetic/phys", "/synthetic/input"
	n := device.Node{Path: "/synthetic/hidraw2", USBParent: &parent, Interface: &device.Interface{Number: 4, Class: 3}, Roles: []string{"physical-buttons"}, Admission: "admitted", Access: "readable", Name: &name, Serial: &serial, PhysicalPath: &physical, SysfsPath: &sysfs, ReportDescriptor: &device.ReportDescriptor{UsagePage: 0xff01, Usage: 0x101, ReportBytes: 64}, USBID: &device.USBID{Vendor: 0x16d0, Product: 0x12f7, Release: 0x111}}
	excluded := device.Node{Path: "/synthetic/hidraw3", USBParent: &parent, Roles: []string{}, Admission: "unsupported", Access: "not_checked"}
	return &device.Result{Groups: []device.Group{{USBParent: parent, HIDPaths: []string{n.Path}}}, Nodes: []device.Node{n, excluded}, Diagnostics: []device.Diagnostic{{Code: device.ERR_DEVICE_UNSUPPORTED, Severity: "warning", Stage: "admission", Target: &excluded.Path}}}
}

func TestDevicesReportProjection(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"devices list", "devices inspect"} {
		t.Run(command, func(t *testing.T) {
			r := projectDevicesReport(command, devicesFixture(), nil)
			var out, errOut bytes.Buffer
			if status := writeDevicesReport(r, true, &out, &errOut); status != 0 || errOut.Len() != 0 || !bytes.HasSuffix(out.Bytes(), []byte("\n")) {
				t.Fatalf("status=%d err=%q", status, &errOut)
			}
			var envelope struct {
				SchemaVersion int  `json:"schema_version"`
				OK            bool `json:"ok"`
				Result        struct {
					Groups      []json.RawMessage            `json:"groups"`
					Nodes       []map[string]json.RawMessage `json:"nodes"`
					Diagnostics []json.RawMessage            `json:"diagnostics"`
				} `json:"result"`
				Error *json.RawMessage `json:"error"`
			}
			decoder := json.NewDecoder(&out)
			if err := decoder.Decode(&envelope); err != nil {
				t.Fatal(err)
			}
			var extra json.RawMessage
			if err := decoder.Decode(&extra); err != io.EOF {
				t.Fatalf("extra JSON: %v", err)
			}
			if envelope.SchemaVersion != 2 || !envelope.OK || envelope.Error != nil || len(envelope.Result.Groups) != 1 || len(envelope.Result.Diagnostics) != 1 {
				t.Fatalf("envelope=%+v", envelope)
			}
			nodes := envelope.Result.Nodes
			if string(nodes[0]["interface"]) != `"04"` {
				t.Fatalf("interface=%s", nodes[0]["interface"])
			}
			if command == "devices list" {
				if len(nodes) != 1 {
					t.Fatal("list exposed excluded node")
				}
				for _, field := range []string{"name", "serial", "report_descriptor", "usb_id", "sysfs_path", "physical_path", "interface_descriptor", "capabilities"} {
					if _, exists := nodes[0][field]; exists {
						t.Fatalf("list exposed %s", field)
					}
				}
			} else {
				if len(nodes) != 2 || string(nodes[0]["serial"]) != `"synthetic-secret"` {
					t.Fatalf("inspect nodes=%v", nodes)
				}
				var descriptor device.ReportDescriptor
				if err := json.Unmarshal(nodes[0]["report_descriptor"], &descriptor); err != nil {
					t.Fatal(err)
				}
				if descriptor.ReportBytes != 64 || descriptor.Numbered || descriptor.UsagePage != 0xff01 || descriptor.Usage != 0x101 {
					t.Fatalf("descriptor=%+v", descriptor)
				}
				for _, field := range []string{"input_id", "capabilities"} {
					if _, ok := nodes[0][field]; ok {
						t.Fatalf("obsolete field %s", field)
					}
				}

				for _, field := range []string{"name", "serial", "report_descriptor", "usb_id", "sysfs_path", "physical_path", "interface_descriptor"} {
					if string(nodes[1][field]) != "null" {
						t.Fatalf("unknown %s=%s", field, nodes[1][field])
					}
				}
			}
		})
	}
}

func TestDevicesReportPartialFailure(t *testing.T) {
	t.Parallel()
	result := devicesFixture()
	denied := result.Nodes[0]
	denied.Path = "/synthetic/hidraw10"
	denied.Access = "denied"
	result.Nodes = append(result.Nodes, denied)
	result.Diagnostics = append(result.Diagnostics, device.Diagnostic{Code: device.ERR_DEVICE_PERMISSION, Severity: "error", Stage: "probe", Target: &denied.Path})
	second := "/synthetic/usb-b"
	result.Groups = append(result.Groups, device.Group{USBParent: second, HIDPaths: []string{denied.Path}})
	report := projectDevicesReport("devices list", result, nil)
	if report.OK || report.Error == nil || report.Error.Code != device.ERR_DEVICE_PERMISSION || len(report.Result.Groups) != 2 || len(report.Result.Nodes) != 2 {
		t.Fatalf("partial=%+v", report)
	}
	var out, errOut bytes.Buffer
	if status := writeDevicesReport(report, false, &out, &errOut); status != 1 || !strings.Contains(out.String(), "Groups: 2") || !strings.Contains(out.String(), device.ERR_DEVICE_UNSUPPORTED) || strings.Contains(out.String(), device.ERR_DEVICE_PERMISSION) || !strings.Contains(errOut.String(), device.ERR_DEVICE_PERMISSION) {
		t.Fatalf("status=%d out=%q err=%q", status, &out, &errOut)
	}
}

func TestDevicesReportEmptyArrays(t *testing.T) {
	t.Parallel()
	result := &device.Result{Diagnostics: []device.Diagnostic{{Code: device.ERR_DEVICE_NOT_FOUND, Severity: "error", Stage: "discovery"}}}
	var out, errOut bytes.Buffer
	if status := writeDevicesReport(projectDevicesReport("devices list", result, nil), true, &out, &errOut); status != 1 || errOut.Len() != 0 {
		t.Fatalf("status=%d", status)
	}
	var report struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if string(report.Result["groups"]) != "[]" || string(report.Result["nodes"]) != "[]" {
		t.Fatalf("null arrays: %s", out.Bytes())
	}
}

func TestDevicesTextDisclosureAndEscaping(t *testing.T) {
	t.Parallel()
	result := devicesFixture()
	result.Nodes[0].Path = "/synthetic/hidraw2\n\x1b"
	for _, command := range []string{"devices list", "devices inspect"} {
		var out, errOut bytes.Buffer
		if status := writeDevicesReport(projectDevicesReport(command, result, nil), false, &out, &errOut); status != 0 || errOut.Len() != 0 {
			t.Fatalf("status=%d err=%q", status, &errOut)
		}
		text := out.String()
		if strings.ContainsRune(text, '\x1b') || strings.Contains(text, "hidraw2\n") {
			t.Fatalf("unescaped controls: %q", text)
		}
		if command == "devices list" && (strings.Contains(text, "synthetic-secret") || strings.Contains(text, "Synthetic\\nName")) {
			t.Fatal("list disclosed detail")
		}
		if command == "devices inspect" && (!strings.Contains(text, "synthetic-secret") || !strings.Contains(text, `Synthetic\nName\x1b`) || !strings.Contains(text, "16d0:12f7:0111")) {
			t.Fatalf("missing inspect detail: %q", text)
		}
	}
}
