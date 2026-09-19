package diagnostics

import (
	"reflect"
	"testing"
)

func TestExitCode(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		severities []Severity
		want       int
	}{
		{"ok", []Severity{SeverityOK}, 0},
		{"warning", []Severity{SeverityOK, SeverityWarning}, 1},
		{"error", []Severity{SeverityError, SeverityWarning, SeverityOK}, 2},
		{"internal", []Severity{SeverityWarning, SeverityInternalError, SeverityError}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checks := make([]Check, len(tc.severities))
			for i, severity := range tc.severities {
				checks[i] = Check{Category: "session", Severity: severity}
			}
			report := NewReport(checks, nil)
			if report.ExitCode != tc.want {
				t.Fatalf("exit=%d, want %d", report.ExitCode, tc.want)
			}
		})
	}
}

func TestReportCategoryOrder(t *testing.T) {
	t.Parallel()
	want := []string{"session", "libraries", "layer-shell", "monitor", "configuration", "profile-source", "device-discovery", "permissions", "evdev-capabilities", "control-socket"}
	checks := []Check{{Category: "configuration", Code: "first", Severity: SeverityOK}}
	for i := len(want) - 1; i >= 0; i-- {
		checks = append(checks, Check{Category: want[i], Code: "second", Severity: SeverityOK})
	}
	report := NewReport(checks, nil)
	var got []string
	for _, check := range report.Checks {
		if len(got) == 0 || got[len(got)-1] != check.Category {
			got = append(got, check.Category)
		}
	}
	if !reflect.DeepEqual(got, want) || report.Checks[4].Code != "first" || report.Checks[5].Code != "second" {
		t.Fatalf("unexpected ordered checks: %+v", report.Checks)
	}
}
