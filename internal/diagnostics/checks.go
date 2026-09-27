package diagnostics

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/control"
	"github.com/sh4869221b/azerlay/internal/device"
	"github.com/sh4869221b/azerlay/internal/profilesource"
)

const (
	OK_SESSION_ENVIRONMENT       = "OK_SESSION_ENVIRONMENT"
	ERR_SESSION_WAYLAND_REQUIRED = "ERR_SESSION_WAYLAND_REQUIRED"
	WARN_CHECK_NOT_IMPLEMENTED   = "WARN_CHECK_NOT_IMPLEMENTED"
	WARN_CHECK_SKIPPED           = "WARN_CHECK_SKIPPED"
	OK_CONFIGURATION             = "OK_CONFIGURATION"
	OK_PROFILE_SOURCE            = "OK_PROFILE_SOURCE"
	ERR_DIAGNOSTIC_PERMISSION    = "ERR_DIAGNOSTIC_PERMISSION"
	OK_CONTROL_SOCKET            = "OK_CONTROL_SOCKET"
	WARN_CONTROL_NOT_RUNNING     = "WARN_CONTROL_NOT_RUNNING"
	WARN_CONTROL_STALE           = "WARN_CONTROL_STALE"
	WARN_CONTROL_DEGRADED        = "WARN_CONTROL_DEGRADED"
	ERR_DOCTOR_INTERNAL          = "ERR_DOCTOR_INTERNAL"
)

func Collect(ctx context.Context, configPath string, includeBindings bool) Report {
	return collect(ctx, configPath, includeBindings, device.Discover)
}
func collect(ctx context.Context, configPath string, includeBindings bool, discover func() (*device.Result, *device.Diagnostic)) Report {
	checks := make([]Check, 0, 12)
	sessionCode := OK_SESSION_ENVIRONMENT
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		sessionCode = ERR_SESSION_WAYLAND_REQUIRED
	}
	checks = append(checks, finding("session", sessionCode, nil))
	for _, backend := range []struct{ category, summary, remediation string }{
		{"libraries", "Library checks are not implemented.", "Follow library integration in issues #15 and #32."},
		{"layer-shell", "Layer Shell checks are not implemented.", "Follow Layer Shell integration in issue #32."},
		{"monitor", "Monitor checks are not implemented.", "Follow monitor support in issues #41 and #33."},
	} {
		checks = append(checks, Check{Category: backend.category, Code: WARN_CHECK_NOT_IMPLEMENTED, Summary: backend.summary, Remediation: backend.remediation, Severity: SeverityWarning})
	}
	checks = append(checks, deviceChecks(discover)...)
	path, err := config.ResolvePath(configPath)
	target := knownTarget(path)
	var settings config.Config
	var warnings []config.Warning
	if err == nil {
		settings, warnings, err = config.Load(path)
	}
	var details *ProfileDetails
	if err != nil {
		code := config.ERR_CONFIG_INVALID
		var failure *config.Error
		if errors.As(err, &failure) && failure.Code == config.ERR_CONFIG_NOT_FOUND {
			code = failure.Code
		}
		check := finding("configuration", code, target)
		if errors.Is(err, os.ErrPermission) {
			check.Summary = "Configuration read permission was denied."
			checks = append(checks, finding("permissions", ERR_DIAGNOSTIC_PERMISSION, target))
		}
		checks = append(checks, check, finding("profile-source", WARN_CHECK_SKIPPED, nil))
	} else {
		checks = append(checks, finding("configuration", OK_CONFIGURATION, target))
		if len(warnings) != 0 {
			checks = append(checks, finding("configuration", config.WARN_CONFIG_UNKNOWN_KEY, target))
		}
		var profileChecks []Check
		profileChecks, details = collectProfile(ctx, settings.Profile, includeBindings)
		checks = append(checks, profileChecks...)
	}
	probe, probeErr := control.Probe(ctx)
	checks = append(checks, socketChecks(probe, probeErr)...)
	return NewReport(checks, details)
}

func collectProfile(ctx context.Context, settings config.Profile, includeBindings bool) ([]Check, *ProfileDetails) {
	var source *profilesource.ImportedSource
	var target *string
	if settings.Source != "local" {
		home, err := profilesource.ResolveDataHome()
		if err != nil {
			return []Check{finding("profile-source", profilesource.ERR_PROFILE_STORAGE, nil)}, nil
		}
		target = knownTarget(filepath.Join(home, "azerlay"))
		source = profilesource.NewImportedSource(home)
	}
	selected, p, err := control.ResolveConfiguredProfile(ctx, source, settings)
	if err != nil {
		code := profilesource.ERR_PROFILE_STORAGE
		var sourceError *profilesource.Error
		var controlError *control.Error
		if errors.As(err, &sourceError) && sourceError.Code == profilesource.ERR_PROFILE_NOT_FOUND {
			code = sourceError.Code
		} else if errors.As(err, &controlError) {
			switch controlError.Code {
			case profilesource.ERR_PROFILE_NOT_FOUND, control.ERR_PROFILE_AMBIGUOUS, control.ERR_PROFILE_SOURCE_UNAVAILABLE:
				code = controlError.Code
			}
		}
		checks := []Check{finding("profile-source", code, target)}
		if errors.Is(err, os.ErrPermission) {
			checks = append(checks, finding("permissions", ERR_DIAGNOSTIC_PERMISSION, target))
		}
		return checks, nil
	}
	var details *ProfileDetails
	if includeBindings {
		details = ProjectProfileDetails(selected.ProfileIndex, p)
	}
	return []Check{finding("profile-source", OK_PROFILE_SOURCE, target)}, details
}

func socketChecks(probe control.ProbeResult, err error) []Check {
	target := knownTarget(probe.Target)
	if err != nil {
		code := control.ERR_CONTROL_UNAVAILABLE
		var failure *control.Error
		if errors.As(err, &failure) {
			switch failure.Code {
			case control.ERR_CONTROL_REQUEST, control.ERR_CONTROL_VERSION, control.ERR_CONTROL_METHOD,
				control.ERR_CONTROL_TOO_LARGE, control.ERR_CONTROL_TIMEOUT, control.ERR_CONTROL_UNAVAILABLE,
				control.ERR_CONTROL_PERMISSION, control.ERR_CONTROL_RUNTIME,
				profilesource.ERR_PROFILE_NOT_FOUND, profilesource.ERR_PROFILE_STORAGE,
				control.ERR_PROFILE_AMBIGUOUS, control.ERR_PROFILE_SOURCE_UNAVAILABLE,
				config.ERR_CONFIG_NOT_FOUND, config.ERR_CONFIG_INVALID:
				code = failure.Code
			}
		}
		check := finding("control-socket", code, target)
		check.Severity = SeverityError
		checks := []Check{check}
		if code == control.ERR_CONTROL_PERMISSION {
			checks = append(checks, finding("permissions", ERR_DIAGNOSTIC_PERMISSION, target))
		}
		return checks
	}
	switch probe.State {
	case control.ProbeNotRunning:
		return []Check{finding("control-socket", WARN_CONTROL_NOT_RUNNING, target)}
	case control.ProbeStale:
		return []Check{finding("control-socket", WARN_CONTROL_STALE, target)}
	case control.ProbeRunning:
		checks := []Check{finding("control-socket", OK_CONTROL_SOCKET, target)}
		seen := make(map[string]bool)
		for _, reason := range probe.Status.DegradedReasons {
			if seen[reason.Code] {
				continue
			}
			seen[reason.Code] = true
			code := WARN_CONTROL_DEGRADED
			switch reason.Code {
			case "DEVICE_UNAVAILABLE", "INPUT_METRICS_UNAVAILABLE", "RENDERER_UNAVAILABLE",
				profilesource.ERR_PROFILE_NOT_FOUND, profilesource.ERR_PROFILE_STORAGE,
				control.ERR_PROFILE_AMBIGUOUS, control.ERR_PROFILE_SOURCE_UNAVAILABLE,
				config.ERR_CONFIG_NOT_FOUND, config.ERR_CONFIG_INVALID, config.WARN_CONFIG_UNKNOWN_KEY:
				code = reason.Code
			}
			checks = append(checks, finding("control-socket", code, target))
		}
		return checks
	default:
		return []Check{finding("control-socket", ERR_DOCTOR_INTERNAL, nil)}
	}
}

func knownTarget(path string) *string {
	if path == "" {
		return nil
	}
	return &path
}

func finding(category, code string, target *string) Check {
	check := Check{Category: category, Code: code, Target: target, Severity: SeverityWarning}
	switch code {
	case OK_SESSION_ENVIRONMENT:
		check.Severity, check.Summary = SeverityOK, "Wayland environment is present; connectivity was not checked."
	case ERR_SESSION_WAYLAND_REQUIRED:
		check.Severity, check.Summary, check.Remediation = SeverityError, "Wayland environment is unavailable.", "Run from a Wayland session."
	case OK_CONFIGURATION:
		check.Severity, check.Summary = SeverityOK, "Configuration loaded."
	case config.WARN_CONFIG_UNKNOWN_KEY:
		check.Summary, check.Remediation = "Configuration contains unsupported keys.", "Check the supported configuration keys."
	case config.ERR_CONFIG_NOT_FOUND:
		check.Severity, check.Summary, check.Remediation = SeverityError, "Configuration was not found.", "Create a valid configuration with schema_version = 1."
	case config.ERR_CONFIG_INVALID:
		check.Severity, check.Summary, check.Remediation = SeverityError, "Configuration could not be loaded.", "Correct the configuration syntax and supported values."
	case WARN_CHECK_SKIPPED:
		check.Summary, check.Remediation = "Configured profile checks were skipped.", "Resolve the configuration error first."
	case OK_PROFILE_SOURCE:
		check.Severity, check.Summary = SeverityOK, "Configured profile for the next start is available."
	case profilesource.ERR_PROFILE_NOT_FOUND:
		check.Summary, check.Remediation = "Profile selection was not found.", "Import a supported export and select its ordinal, or correct profile.selected_id."
	case control.ERR_PROFILE_AMBIGUOUS:
		check.Summary, check.Remediation = "Profile selection is ambiguous.", "Choose a unique configured profile ID."
	case control.ERR_PROFILE_SOURCE_UNAVAILABLE:
		check.Summary, check.Remediation = "Local profile source is unavailable.", "Use a supported imported source while local source support awaits issue #22."
	case profilesource.ERR_PROFILE_STORAGE:
		check.Severity, check.Summary, check.Remediation = SeverityError, "Profile storage could not be read.", "Inspect profile storage access and integrity; doctor does not repair it."
	case ERR_DIAGNOSTIC_PERMISSION:
		check.Severity, check.Summary, check.Remediation = SeverityError, "Access permission was denied.", "Check current-user read access and private ownership of the target."
	case OK_CONTROL_SOCKET:
		check.Severity, check.Summary = SeverityOK, "Control socket connectivity succeeded."
	case WARN_CONTROL_NOT_RUNNING:
		check.Summary, check.Remediation = "Control instance is not running.", "Start the application with azerlay run."
	case WARN_CONTROL_STALE:
		check.Summary, check.Remediation = "Control socket refused the connection and appears stale.", "Start with azerlay run for owned stale recovery; doctor does not remove the socket."
	case control.ERR_CONTROL_RUNTIME:
		check.Severity, check.Summary, check.Remediation = SeverityError, "Control runtime is unavailable or unsafe.", "Repair the same-user private runtime directory settings."
	case control.ERR_CONTROL_PERMISSION:
		check.Severity, check.Summary, check.Remediation = SeverityError, "Control socket access is denied or unsafe.", "Use the same user and private runtime permissions."
	case control.ERR_CONTROL_REQUEST, control.ERR_CONTROL_VERSION, control.ERR_CONTROL_METHOD,
		control.ERR_CONTROL_TOO_LARGE, control.ERR_CONTROL_TIMEOUT, control.ERR_CONTROL_UNAVAILABLE:
		check.Severity, check.Summary, check.Remediation = SeverityError, "Control status exchange failed.", "Check or restart the matching application."
	case "DEVICE_UNAVAILABLE":
		check.Summary, check.Remediation = "Running instance reports an unavailable device.", "Check supported device setup; follow issues #13 and #27."
	case "INPUT_METRICS_UNAVAILABLE":
		check.Summary, check.Remediation = "Running instance reports unavailable input metrics.", "Follow input support in issues #27 and #28."
	case "RENDERER_UNAVAILABLE":
		check.Summary, check.Remediation = "Running instance reports an unavailable renderer.", "Follow renderer support in issue #32."
	case WARN_CONTROL_DEGRADED:
		check.Summary, check.Remediation = "Running instance reports a degraded condition.", "Check or restart the matching application."
	default:
		check.Code, check.Severity, check.Target = ERR_DOCTOR_INTERNAL, SeverityInternalError, nil
		check.Summary, check.Remediation = "Doctor encountered an internal diagnostic error.", "Report the diagnostic code and application version."
	}
	return check
}
