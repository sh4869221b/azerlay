package diagnostics

import "github.com/sh4869221b/azerlay/internal/device"

const (
	OK_DEVICE_DISCOVERY    = "OK_DEVICE_DISCOVERY"
	OK_DEVICE_ACCESS       = "OK_DEVICE_ACCESS"
	OK_HIDRAW_CAPABILITIES = "OK_HIDRAW_CAPABILITIES"
)

func deviceChecks(discover func() (*device.Result, *device.Diagnostic)) []Check {
	result, failure := discover()
	categories := []string{"device-discovery", "permissions", "hidraw-capabilities"}
	if failure != nil {
		checks := []Check{rawFinding(categories[0], failure.Code, nil)}
		for _, category := range categories[1:] {
			checks = append(checks, finding(category, WARN_CHECK_SKIPPED, nil))
		}
		return checks
	}
	checks := []Check{}
	switch len(result.Groups) {
	case 0:
		checks = append(checks, rawFinding(categories[0], device.ERR_DEVICE_NOT_FOUND, nil))
	case 1:
		code := OK_DEVICE_DISCOVERY
		if !result.Groups[0].Complete {
			code = device.ERR_DEVICE_AMBIGUOUS
		}
		checks = append(checks, rawFinding(categories[0], code, nil))
	default:
		checks = append(checks, rawFinding(categories[0], device.ERR_DEVICE_AMBIGUOUS, nil))
	}
	admitted := 0
	for _, node := range result.Nodes {
		if node.Admission != "admitted" {
			code := device.ERR_DEVICE_UNSUPPORTED
			if node.Admission == "indeterminate" {
				code = device.ERR_DEVICE_METADATA
			}
			checks = append(checks, rawFinding("hidraw-capabilities", code, &node.Path))
			continue
		}
		admitted++
		checks = append(checks, rawFinding("hidraw-capabilities", OK_HIDRAW_CAPABILITIES, &node.Path))
		code := OK_DEVICE_ACCESS
		switch node.Access {
		case "readable":
		case "denied":
			code = device.ERR_DEVICE_PERMISSION
		default:
			code = device.ERR_DEVICE_DISCONNECTED
		}
		checks = append(checks, rawFinding("permissions", code, &node.Path))
	}
	if admitted == 0 {
		checks = append(checks, finding("permissions", WARN_CHECK_SKIPPED, nil))
	}
	if len(result.Nodes) == 0 {
		checks = append(checks, finding("hidraw-capabilities", WARN_CHECK_SKIPPED, nil))
	}
	return checks
}

// Only fixed diagnostics and node paths cross this projection. USB serials,
// arbitrary source summaries and raw reports never enter doctor output.
func rawFinding(category, code string, target *string) Check {
	check := Check{Category: category, Code: code, Target: target, Severity: SeverityError}
	switch code {
	case OK_DEVICE_DISCOVERY:
		check.Severity = SeverityOK
		check.Summary = "One qualified physical-button interface was found."
	case OK_DEVICE_ACCESS:
		check.Severity = SeverityOK
		check.Summary = "Read-only hidraw access succeeded; reports were not consumed."
	case OK_HIDRAW_CAPABILITIES:
		check.Severity = SeverityOK
		check.Summary = "The physical-button report descriptor is supported."
	case device.ERR_DEVICE_NOT_FOUND:
		check.Severity = SeverityWarning
		check.Summary = "No qualified physical-button interface was found."
	case device.ERR_DEVICE_AMBIGUOUS:
		check.Summary = "More than one physical-button interface matches."
	case device.ERR_DEVICE_PERMISSION:
		check.Summary = "Read-only hidraw access was denied."
	case device.ERR_DEVICE_UNSUPPORTED:
		check.Severity = SeverityWarning
		check.Summary = "HID identity or report descriptor is unsupported."
	case device.ERR_DEVICE_DISCONNECTED:
		check.Summary = "The hidraw interface is unavailable."
	default:
		check.Code = device.ERR_DEVICE_METADATA
		check.Summary = "HID metadata could not be established."
	}
	check.Remediation = "Check the supported device and hidraw access. Start the official software in SOFTWARE mode; notification initialization is not checked."
	return check
}
