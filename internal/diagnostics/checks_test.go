package diagnostics

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/control"
	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profilesource"
)

const collectBundle = `{"profiles":[{"id":"PRIVATE_ID","name":"PRIVATE_NAME","inputs":[{"label":"SELECTED_LABEL","macro":"PRIVATE_MACRO"}]},{"id":"other","inputs":[{"label":"OTHER_LABEL"}]}]}`

func TestCollectLivePrivacy(t *testing.T) {
	for _, include := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "opt in"}[include], func(t *testing.T) {
			path, home := collectFixture(t)
			importCollectProfile(t, home, collectBundle)
			serveCollectStatus(t)
			before, err := os.ReadFile(filepath.Join(home, "azerlay/cache/source-index.json"))
			if err != nil {
				t.Fatal(err)
			}
			report := Collect(t.Context(), path, include)
			if report.ExitCode != 1 || (report.ProfileDetails != nil) != include {
				t.Fatalf("report=%+v", report)
			}
			if include && (report.ProfileDetails.ProfileIndex != 1 || *report.ProfileDetails.Controls[0].Label != "SELECTED_LABEL") {
				t.Fatalf("details=%+v", report.ProfileDetails)
			}
			assertCollectCategories(t, report)
			counts := make(map[string]int)
			for _, check := range report.Checks {
				counts[check.Code]++
			}
			if counts[WARN_CHECK_NOT_IMPLEMENTED] != 6 || counts[config.WARN_CONFIG_UNKNOWN_KEY] != 1 || counts[OK_CONTROL_SOCKET] != 1 || counts["DEVICE_UNAVAILABLE"] != 1 || counts[WARN_CONTROL_DEGRADED] != 1 {
				t.Fatalf("codes=%v", counts)
			}
			for _, jsonMode := range []bool{false, true} {
				var output bytes.Buffer
				if err := WriteReport(&output, report, jsonMode); err != nil {
					t.Fatal(err)
				}
				for _, private := range []string{"PRIVATE", "OTHER_LABEL"} {
					if strings.Contains(output.String(), private) {
						t.Fatalf("leaked %s: %s", private, output.String())
					}
				}
				if strings.Contains(output.String(), "SELECTED_LABEL") != include {
					t.Fatalf("disclosure mismatch: %s", output.String())
				}
			}
			after, err := os.ReadFile(filepath.Join(home, "azerlay/cache/source-index.json"))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("selection changed: %v", err)
			}
		})
	}
}

func TestCollectFailures(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		exit       int
	}{
		{"missing config", config.ERR_CONFIG_NOT_FOUND, 2},
		{"invalid config", config.ERR_CONFIG_INVALID, 2},
		{"missing profile", profilesource.ERR_PROFILE_NOT_FOUND, 1},
		{"local", control.ERR_PROFILE_SOURCE_UNAVAILABLE, 1},
		{"ambiguous", control.ERR_PROFILE_AMBIGUOUS, 1},
		{"corrupt", profilesource.ERR_PROFILE_STORAGE, 2},
		{"data path", profilesource.ERR_PROFILE_STORAGE, 2},
		{"config permission", ERR_DIAGNOSTIC_PERMISSION, 2},
		{"store permission", ERR_DIAGNOSTIC_PERMISSION, 2},
		{"runtime", control.ERR_CONTROL_RUNTIME, 2},
		{"session", ERR_SESSION_WAYLAND_REQUIRED, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, home := collectFixture(t)
			var err error
			switch tc.name {
			case "missing config":
				err = os.Remove(path)
			case "invalid config":
				err = os.WriteFile(path, []byte("PRIVATE_INVALID=["), 0600)
			case "local":
				err = os.WriteFile(path, []byte("schema_version=1\n[profile]\nsource='local'\n"), 0600)
				t.Setenv("HOME", "")
				t.Setenv("XDG_DATA_HOME", "")
			case "ambiguous":
				importCollectProfile(t, home, `{"profiles":[{"id":"PRIVATE_ID","inputs":[]},{"id":"PRIVATE_ID","inputs":[]}]}`)
			case "corrupt":
				selected := importCollectProfile(t, home, collectBundle)
				err = os.WriteFile(filepath.Join(home, "azerlay/sources", selected.Source.Hash+".azeron"), []byte("PRIVATE_CORRUPTION"), 0600)
			case "data path":
				t.Setenv("HOME", "")
				t.Setenv("XDG_DATA_HOME", "")
			case "config permission", "store permission":
				if os.Geteuid() == 0 {
					t.Fatal("permission-denied fixture requires an unprivileged process")
				}
				denied := path
				mode := os.FileMode(0600)
				if tc.name == "store permission" {
					importCollectProfile(t, home, collectBundle)
					denied, mode = home, 0700
				}
				err = os.Chmod(denied, 0)
				t.Cleanup(func() {
					if err := os.Chmod(denied, mode); err != nil {
						t.Error(err)
					}
				})
			case "runtime":
				t.Setenv("XDG_RUNTIME_DIR", "relative")
			case "session":
				t.Setenv("WAYLAND_DISPLAY", "")
			}
			if err != nil {
				t.Fatal(err)
			}
			report := Collect(t.Context(), path, true)
			assertCollectCategories(t, report)
			if report.ExitCode != tc.exit || report.ProfileDetails != nil {
				t.Fatalf("report=%+v", report)
			}
			found, skipped, socket := false, false, false
			for _, check := range report.Checks {
				found = found || check.Code == tc.code
				skipped = skipped || check.Code == WARN_CHECK_SKIPPED
				socket = socket || check.Category == "control-socket"
				if tc.name == "local" && check.Category == "profile-source" && check.Target != nil {
					t.Fatal("local source exposed a store target")
				}
			}
			if !found || !socket || (strings.Contains(tc.name, "config") && !skipped) {
				t.Fatalf("checks=%+v", report.Checks)
			}
		})
	}
}

func TestCollectSocketProjection(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		result   control.ProbeResult
		err      error
		code     string
		severity Severity
	}{
		{"absent", control.ProbeResult{State: control.ProbeNotRunning}, nil, WARN_CONTROL_NOT_RUNNING, SeverityWarning},
		{"stale", control.ProbeResult{State: control.ProbeStale}, nil, WARN_CONTROL_STALE, SeverityWarning},
		{"timeout", control.ProbeResult{}, control.NewError(control.ERR_CONTROL_TIMEOUT), control.ERR_CONTROL_TIMEOUT, SeverityError},
		{"permission", control.ProbeResult{}, control.NewError(control.ERR_CONTROL_PERMISSION), control.ERR_CONTROL_PERMISSION, SeverityError},
		{"unknown error", control.ProbeResult{}, &control.Error{Code: "PRIVATE_CODE", Summary: "PRIVATE_SUMMARY"}, control.ERR_CONTROL_UNAVAILABLE, SeverityError},
		{"internal", control.ProbeResult{}, nil, ERR_DOCTOR_INTERNAL, SeverityInternalError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checks := socketChecks(tc.result, tc.err)
			if checks[0].Code != tc.code || checks[0].Severity != tc.severity {
				t.Fatalf("checks=%+v", checks)
			}
			if tc.name == "permission" && (len(checks) != 2 || checks[1].Code != ERR_DIAGNOSTIC_PERMISSION) {
				t.Fatalf("permission checks=%+v", checks)
			}
		})
	}
	for _, code := range []string{profilesource.ERR_PROFILE_STORAGE, config.ERR_CONFIG_INVALID, config.ERR_CONFIG_NOT_FOUND} {
		checks := socketChecks(control.ProbeResult{State: control.ProbeRunning, Status: &control.Status{DegradedReasons: []control.Diagnostic{{Code: code, Reason: "PRIVATE_REASON"}}}}, nil)
		if len(checks) != 2 || checks[1].Code != code || checks[1].Severity != SeverityError || strings.Contains(checks[1].Summary, "PRIVATE") {
			t.Fatalf("live error checks=%+v", checks)
		}
	}
}

func collectFixture(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	runtime, err := os.MkdirTemp("", "az-doc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(runtime); err != nil {
			t.Error(err)
		}
	})
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", home)
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	t.Setenv("WAYLAND_DISPLAY", "wayland-test")
	path := filepath.Join(home, "config.toml")
	if err := os.WriteFile(path, []byte("schema_version=1\nPRIVATE_KEY='PRIVATE_VALUE'\nPRIVATE_SECOND=true\n[profile]\nselected_id='PRIVATE_ID'\n[diagnostics]\ninclude_bindings=true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return path, home
}

func importCollectProfile(t *testing.T, home, input string) profilesource.Selection {
	t.Helper()
	prepared, err := profilesource.PrepareText(input, profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export"})
	if err != nil {
		t.Fatal(err)
	}
	selected, err := profilesource.NewImportedSource(home).Import(t.Context(), prepared, len(prepared.Bundle().Profiles), profilesource.Origin{Kind: "text"})
	if err != nil {
		t.Fatal(err)
	}
	return selected
}

func assertCollectCategories(t *testing.T, report Report) {
	t.Helper()
	var got []string
	for _, check := range report.Checks {
		if check.Severity != SeverityOK && check.Remediation == "" {
			t.Fatalf("missing remediation: %+v", check)
		}
		if len(got) == 0 || got[len(got)-1] != check.Category {
			got = append(got, check.Category)
		}
	}
	want := []string{"session", "libraries", "layer-shell", "monitor", "configuration", "profile-source", "device-discovery", "permissions", "evdev-capabilities", "control-socket"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("categories=%v", got)
	}
}

func serveCollectStatus(t *testing.T) {
	t.Helper()
	path := filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "azerlay/control.sock")
	if err := os.Mkdir(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		listener.Close()
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		conn, err := listener.AcceptUnix()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
			done <- err
			return
		}
		request, err := control.ReadRequest(conn)
		if err != nil {
			done <- err
			return
		}
		status := control.Status{SchemaVersion: 1, Generation: 1, EventNodes: []string{}, ActiveProfile: &control.ProfileStatus{Source: "imported", SourceRef: "PRIVATE_HASH", ProfileIndex: 2, Name: stringPointer("PRIVATE_LIVE_NAME")}, DegradedReasons: []control.Diagnostic{{Code: "DEVICE_UNAVAILABLE", Stage: "PRIVATE_STAGE", Reason: "PRIVATE_REASON"}, {Code: "DEVICE_UNAVAILABLE", Stage: "other", Reason: "PRIVATE_REASON"}, {Code: "PRIVATE_UNKNOWN_CODE", Stage: "other", Reason: "PRIVATE_REASON"}}}
		data, err := control.EncodeResponse(control.SuccessResponse(request, status))
		if err == nil {
			_, err = conn.Write(data)
		}
		done <- err
	}()
	t.Cleanup(func() {
		listener.Close()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("status peer did not stop")
		}
	})
}
