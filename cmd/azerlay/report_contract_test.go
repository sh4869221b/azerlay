package main

import (
	"encoding/json"

	"github.com/sh4869221b/azerlay/internal/device"
	"github.com/sh4869221b/azerlay/internal/profile"
)

// These test-local wire contracts intentionally retain the pre-extraction JSON
// field types. Native integration tests must not depend on CLI-private DTOs.
type operationReport struct {
	SchemaVersion int            `json:"schema_version"`
	Command       string         `json:"command"`
	OK            bool           `json:"ok"`
	Result        *profileReport `json:"result"`
	Error         *reportError   `json:"error"`
}

type reportError struct {
	Code        string `json:"code"`
	Stage       string `json:"stage"`
	Summary     string `json:"summary"`
	Remediation string `json:"remediation"`
}

type profileReport struct {
	RootKind             profile.RootKind `json:"root_kind"`
	ExportVersion        exportVersion    `json:"export_version"`
	Profiles             []profileRow     `json:"profiles"`
	SelectedProfileIndex *int             `json:"selected_profile_index"`
	Warnings             []profileWarning `json:"warnings"`
}

type exportVersion struct {
	Present bool            `json:"present"`
	Value   json.RawMessage `json:"value"`
}

type profileRow struct {
	Index      int     `json:"index"`
	Name       *string `json:"name"`
	InputCount int     `json:"input_count"`
}

type profileWarning struct {
	Code         string `json:"code"`
	ProfileIndex int    `json:"profile_index"`
	Count        int    `json:"count"`
}

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
	USBParent string   `json:"usb_parent"`
	Complete  bool     `json:"complete"`
	HIDPaths  []string `json:"hidraw_paths"`
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

type devicesDetails struct {
	Name                *string                  `json:"name"`
	ReportDescriptor    *device.ReportDescriptor `json:"report_descriptor"`
	USBID               *devicesUSBID            `json:"usb_id"`
	SysfsPath           *string                  `json:"sysfs_path"`
	PhysicalPath        *string                  `json:"physical_path"`
	Serial              *string                  `json:"serial"`
	InterfaceDescriptor *devicesInterface        `json:"interface_descriptor"`
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
