package device

import (
	"errors"
	"io/fs"
	"syscall"
)

const (
	ERR_DEVICE_NOT_FOUND    = "ERR_DEVICE_NOT_FOUND"
	ERR_DEVICE_PERMISSION   = "ERR_DEVICE_PERMISSION"
	ERR_DEVICE_DISCONNECTED = "ERR_DEVICE_DISCONNECTED"
	ERR_DEVICE_UNSUPPORTED  = "ERR_DEVICE_UNSUPPORTED"
	ERR_DEVICE_METADATA     = "ERR_DEVICE_METADATA"
	ERR_DEVICE_AMBIGUOUS    = "ERR_DEVICE_AMBIGUOUS"
	WARN_DEVICE_INCOMPLETE  = "WARN_DEVICE_INCOMPLETE"
	WARN_DEVICE_ROOT        = "WARN_DEVICE_ROOT"
)

type Diagnostic struct {
	Code        string  `json:"code"`
	Severity    string  `json:"severity"`
	Stage       string  `json:"stage"`
	Summary     string  `json:"summary"`
	Target      *string `json:"target"`
	Remediation string  `json:"remediation"`
}

func (d *Diagnostic) Error() string { return d.Code }

func diagnostic(code, stage string, target *string) Diagnostic {
	d := Diagnostic{Code: code, Severity: "error", Stage: stage, Target: target}
	switch code {
	case ERR_DEVICE_NOT_FOUND:
		d.Summary, d.Remediation = "No qualifying device was found.", "Connect a supported device and check its hidraw path."
	case ERR_DEVICE_PERMISSION:
		d.Summary, d.Remediation = "Device access was denied.", "Check the active seat/session and specific supported-device uaccess packaging, then reconnect the device."
	case ERR_DEVICE_DISCONNECTED:
		d.Summary, d.Remediation = "Device disappeared during discovery.", "Reconnect the device and enumerate it again."
	case ERR_DEVICE_UNSUPPORTED:
		d.Summary, d.Remediation = "Device identity or report descriptor are unsupported.", "Check the researched Cyborg II identity contract."
	case ERR_DEVICE_AMBIGUOUS:
		d.Summary, d.Remediation = "More than one device or selected interface matches.", "Select a unique device and reconnect it."
	case WARN_DEVICE_INCOMPLETE:
		d.Severity, d.Summary, d.Remediation = "warning", "Some expected device interfaces are missing or excluded.", "Inspect the device nodes and check connection and unsupported-node diagnostics."
	case WARN_DEVICE_ROOT:
		d.Severity, d.Summary, d.Remediation = "warning", "Root readability does not establish ordinary-user access.", "Run as the ordinary active-session user to check access."
	default:
		d.Summary, d.Remediation = "Device metadata could not be established.", "Check the device connection and inspect its supported identity and metadata."
	}
	return d
}

func boundaryDiagnostic(err error, stage string, target *string) Diagnostic {
	code := ERR_DEVICE_METADATA
	if errors.Is(err, errUnsupported) {
		code = ERR_DEVICE_UNSUPPORTED
	}
	if errors.Is(err, fs.ErrPermission) || errors.Is(err, syscall.EPERM) {
		code = ERR_DEVICE_PERMISSION
	} else if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENODEV) || errors.Is(err, syscall.ENXIO) {
		code = ERR_DEVICE_DISCONNECTED
	}
	return diagnostic(code, stage, target)
}

func ReconnectDiagnostic(err error) Diagnostic {
	return boundaryDiagnostic(err, "reconnect", nil)
}

var errUnsupported = errors.New("unsupported HID node")
