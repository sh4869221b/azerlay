package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/sh4869221b/azerlay/internal/device"
)

type devicesReport struct {
	SchemaVersion int                `json:"schema_version"`
	Command       string             `json:"command"`
	OK            bool               `json:"ok"`
	Result        *devicesResult     `json:"result"`
	Error         *device.Diagnostic `json:"error"`
}

type devicesResult struct {
	Groups      []devicesGroup      `json:"groups"`
	Nodes       []devicesNode       `json:"nodes"`
	Diagnostics []device.Diagnostic `json:"diagnostics"`
}

type devicesGroup struct {
	USBParent  string   `json:"usb_parent"`
	Complete   bool     `json:"complete"`
	EventPaths []string `json:"event_paths"`
}

type devicesNode struct {
	Path      string   `json:"path"`
	USBParent *string  `json:"usb_parent"`
	Interface *string  `json:"interface"`
	Roles     []string `json:"roles"`
	Admission string   `json:"admission"`
	Access    string   `json:"access"`
	*devicesDetails
}

// Only inspect projection allocates details; list cannot encode these fields.
type devicesDetails struct {
	Name                *string           `json:"name"`
	InputID             *devicesInputID   `json:"input_id"`
	USBID               *devicesUSBID     `json:"usb_id"`
	SysfsPath           *string           `json:"sysfs_path"`
	PhysicalPath        *string           `json:"physical_path"`
	Serial              *string           `json:"serial"`
	InterfaceDescriptor *devicesInterface `json:"interface_descriptor"`
	Capabilities        map[string][]int  `json:"capabilities"`
}

type devicesInputID struct {
	Bus     string `json:"bus"`
	Vendor  string `json:"vendor"`
	Product string `json:"product"`
	Version string `json:"version"`
}

type devicesUSBID struct {
	Vendor  string `json:"vendor"`
	Product string `json:"product"`
	Release string `json:"release"`
}

type devicesInterface struct {
	Class    string `json:"class"`
	Subclass string `json:"subclass"`
	Protocol string `json:"protocol"`
}

func projectDevicesReport(command string, result *device.Result, failure *device.Diagnostic) devicesReport {
	r := devicesReport{SchemaVersion: 1, Command: command, Error: failure}
	if result == nil {
		return r
	}
	r.OK = result.OK() && failure == nil
	r.Result = &devicesResult{Groups: []devicesGroup{}, Nodes: []devicesNode{}, Diagnostics: append([]device.Diagnostic{}, result.Diagnostics...)}
	for _, g := range result.Groups {
		r.Result.Groups = append(r.Result.Groups, devicesGroup{g.USBParent, g.Complete, append([]string{}, g.EventPaths...)})
	}
	for _, n := range result.Nodes {
		if command == "devices list" && n.Admission != "admitted" {
			continue
		}
		row := devicesNode{Path: n.Path, USBParent: n.USBParent, Roles: append([]string{}, n.Roles...), Admission: n.Admission, Access: n.Access}
		if n.Interface != nil {
			value := fmt.Sprintf("%02x", n.Interface.Number)
			row.Interface = &value
		}
		if command == "devices inspect" {
			d := &devicesDetails{Name: n.Name, SysfsPath: n.SysfsPath, PhysicalPath: n.PhysicalPath, Serial: n.Serial, Capabilities: n.Capabilities}
			if n.InputID != nil {
				id := n.InputID
				d.InputID = &devicesInputID{fmt.Sprintf("%04x", id.Bus), fmt.Sprintf("%04x", id.Vendor), fmt.Sprintf("%04x", id.Product), fmt.Sprintf("%04x", id.Version)}
			}
			if n.USBID != nil {
				id := n.USBID
				d.USBID = &devicesUSBID{fmt.Sprintf("%04x", id.Vendor), fmt.Sprintf("%04x", id.Product), fmt.Sprintf("%04x", id.Release)}
			}
			if n.Interface != nil {
				i := n.Interface
				d.InterfaceDescriptor = &devicesInterface{fmt.Sprintf("%02x", i.Class), fmt.Sprintf("%02x", i.Subclass), fmt.Sprintf("%02x", i.Protocol)}
			}
			row.devicesDetails = d
		}
		r.Result.Nodes = append(r.Result.Nodes, row)
	}
	if r.Error == nil {
		for i := range r.Result.Diagnostics {
			if r.Result.Diagnostics[i].Severity == "error" {
				r.Error = &r.Result.Diagnostics[i]
				break
			}
		}
	}
	return r
}

func writeDevicesReport(report devicesReport, jsonMode bool, stdout, stderr io.Writer) int {
	status := 1
	if report.OK {
		status = 0
	}
	if report.Error != nil && report.Error.Stage == "usage" {
		status = 2
	}
	if jsonMode {
		if err := json.NewEncoder(stdout).Encode(report); err != nil {
			return 1
		}
		return status
	}
	var output, failures strings.Builder
	if report.Result != nil {
		fmt.Fprintf(&output, "Groups: %d\n", len(report.Result.Groups))
		for _, g := range report.Result.Groups {
			fmt.Fprintf(&output, "  USB parent: %q; complete: %t; events: %q\n", g.USBParent, g.Complete, g.EventPaths)
		}
		for _, n := range report.Result.Nodes {
			fmt.Fprintf(&output, "Node: %q; USB parent: %s; interface: %s; roles: %q; %s; %s\n", n.Path, deviceText(n.USBParent), deviceText(n.Interface), n.Roles, n.Admission, n.Access)
			if n.devicesDetails != nil {
				renderDeviceDetails(&output, n.devicesDetails)
			}
		}
		for _, d := range report.Result.Diagnostics {
			if d.Severity == "error" {
				renderDeviceDiagnostic(&failures, d)
			} else {
				renderDeviceDiagnostic(&output, d)
			}
		}
	} else if report.Error != nil {
		renderDeviceDiagnostic(&failures, *report.Error)
	}
	if output.Len() > 0 {
		if _, err := io.WriteString(stdout, output.String()); err != nil {
			return 1
		}
	}
	if failures.Len() > 0 {
		if _, err := io.WriteString(stderr, failures.String()); err != nil {
			return 1
		}
	}
	return status
}

func deviceText(value *string) string {
	if value == nil {
		return "<unknown>"
	}
	return strconv.Quote(*value)
}

func renderDeviceDiagnostic(out *strings.Builder, d device.Diagnostic) {
	fmt.Fprintf(out, "%s (%s) target=%s: %s\n%s\n", d.Code, d.Stage, deviceText(d.Target), d.Summary, d.Remediation)
}

func renderDeviceDetails(out *strings.Builder, d *devicesDetails) {
	fmt.Fprintf(out, "  Name: %s\n  Sysfs path: %s\n  Physical path: %s\n  Serial: %s\n", deviceText(d.Name), deviceText(d.SysfsPath), deviceText(d.PhysicalPath), deviceText(d.Serial))
	if d.InputID != nil {
		fmt.Fprintf(out, "  Input ID: %s:%s:%s:%s\n", d.InputID.Bus, d.InputID.Vendor, d.InputID.Product, d.InputID.Version)
	} else {
		out.WriteString("  Input ID: <unknown>\n")
	}
	if d.USBID != nil {
		fmt.Fprintf(out, "  USB ID: %s:%s:%s\n", d.USBID.Vendor, d.USBID.Product, d.USBID.Release)
	} else {
		out.WriteString("  USB ID: <unknown>\n")
	}
	if d.InterfaceDescriptor != nil {
		i := d.InterfaceDescriptor
		fmt.Fprintf(out, "  Interface descriptor: %s/%s/%s\n", i.Class, i.Subclass, i.Protocol)
	} else {
		out.WriteString("  Interface descriptor: <unknown>\n")
	}
	for _, family := range []string{"ev", "key", "rel", "abs", "msc", "sw", "led", "ff", "snd"} {
		if codes, ok := d.Capabilities[family]; ok {
			fmt.Fprintf(out, "  Capabilities %s: %v\n", family, codes)
		}
	}
}
