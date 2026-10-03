package main

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/control"
	"github.com/sh4869221b/azerlay/internal/diagnostics"
	"github.com/sh4869221b/azerlay/internal/profilesource"
)

const doctorCLIBundle = `{"profiles":[{"id":"PRIVATE_ID_FIRST","name":"PRIVATE_NAME","inputs":[{"label":"SELECTED_LABEL\n\u001b","types":["1","11","11"],"keyValues":["KeyU","0","0","0"],"metaValues":["ControlLeft","0","0"],"isHold":false,"isTurbo":false,"isToggleOnHold":false,"keyValuesLong":["0","0","0","0"],"metaValuesLong":["0","0","0"],"keyValuesDouble":["0","0","0","0"],"metaValuesDouble":["0","0","0"],"future":"PRIVATE_FIELD"},{"macro":"PRIVATE_MACRO"}]},{"id":"PRIVATE_ID_SECOND","name":"Other profile","inputs":[{"label":"OTHER_LABEL"}]}]}`

func TestCLIDoctorLocalReadOnly(t *testing.T) {
	fixture := newRunFixture(t)
	root := doctorRoot(fixture)
	store := filepath.Join(root, "userData")
	path := filepath.Join(store, "Storage/DevicesStorage/device/ProfileStorage/profile_a.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	data := `{"id":"a","name":"PRIVATE_NAME","version":1,"inputs":[],"isSoftware":true,"metaData":{"changedLogs":[{"softwareVersion":"2.0.2"}]}}`
	cliWrite(t, path, []byte(data), 0600)
	cliWrite(t, fixture.config, []byte("schema_version=1\n[profile]\nsource='local'\nlocal_store_path='"+store+"'\nlocal_device='device'\nlocal_profile_file='profile_a.json'\n"), 0600)
	before := cliTree(t, root)
	output := invokeCLI(t, fixture.env, nil, "doctor", "--config", fixture.config, "--json")
	checkCLIStatus(t, output, 1, true)
	requireDoctorCode(t, parsedDoctorReport(t, output), diagnostics.OK_PROFILE_SOURCE)
	if strings.Contains(output.stdout, "PRIVATE_NAME") || strings.Contains(output.stdout, store) {
		t.Fatalf("private local diagnostics: %s", output.stdout)
	}
	requireDoctorNoWrites(t, root, before)
	cliWrite(t, path, []byte(strings.Replace(data, "2.0.2", "9.9.9", 1)), 0600)
	before = cliTree(t, root)
	output = invokeCLI(t, fixture.env, nil, "doctor", "--config", fixture.config, "--json")
	checkCLIStatus(t, output, 1, true)
	requireDoctorCode(t, parsedDoctorReport(t, output), profilesource.ERR_PROFILE_LOCAL_UNSUPPORTED)
	requireDoctorNoWrites(t, root, before)
}

func seedDoctorCLI(t *testing.T, fixture runFixture) {
	t.Helper()
	output := invokeCLI(t, fixture.env, nil, "import", "--software-release", "2.0.2", "--text", doctorCLIBundle, "--profile-index", "2", "--json")
	checkCLIStatus(t, output, 0, true)
	cliWrite(t, fixture.config, []byte("schema_version=1\nPRIVATE_KEY='PRIVATE_VALUE'\n[profile]\nselected_id='PRIVATE_ID_FIRST'\n[diagnostics]\ninclude_bindings=true\n"), 0600)
}

func TestCLIDoctorLive(t *testing.T) {
	fixture := newRunFixture(t)
	seedDoctorCLI(t, fixture)
	process := startRunProcess(t, fixture)
	readyRunProcess(t, process, fixture)
	controlCLIResult[control.SelectionResult](t, fixture.env, "profiles", "select", "PRIVATE_ID_SECOND")
	controlCLIResult[control.VisibilityResult](t, fixture.env, "show")
	beforeStatus := controlCLIResult[control.Status](t, fixture.env, "status")
	root := doctorRoot(fixture)
	beforeFiles := cliTree(t, root)
	socketInfo, err := os.Lstat(fixture.socket)
	if err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(filepath.Dir(fixture.socket), "instance.lock")
	lockInfo, err := os.Lstat(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"doctor", "--config", fixture.config},
		{"doctor", "--config=" + fixture.config, "--include-bindings", "--json"},
	} {
		output := invokeCLI(t, fixture.env, nil, args...)
		checkCLIStatus(t, output, 1, true)
		if strings.Contains(output.stdout, "PRIVATE") || strings.Contains(output.stdout, "OTHER_LABEL") {
			t.Fatalf("private doctor output: %s", output.stdout)
		}
		if strings.HasPrefix(output.stdout, "{") {
			report := parsedDoctorReport(t, output)
			requireDoctorCode(t, report, diagnostics.OK_CONTROL_SOCKET)
			requireDoctorCode(t, report, "DEVICE_UNAVAILABLE")
			if report.ProfileDetails == nil || report.ProfileDetails.ProfileIndex != 1 || *report.ProfileDetails.Controls[0].Label != "SELECTED_LABEL\n\x1b" {
				t.Fatalf("session override replaced configured profile: %+v", report.ProfileDetails)
			}
		} else if !strings.Contains(output.stdout, diagnostics.OK_CONTROL_SOCKET) || strings.Contains(output.stdout, "SELECTED_LABEL") {
			t.Fatalf("text output=%s", output.stdout)
		}
	}
	afterStatus := controlCLIResult[control.Status](t, fixture.env, "status")
	if beforeStatus.Generation != afterStatus.Generation || beforeStatus.Visible != afterStatus.Visible || !reflect.DeepEqual(beforeStatus.ActiveProfile, afterStatus.ActiveProfile) || !reflect.DeepEqual(beforeStatus.LastReload, afterStatus.LastReload) {
		t.Fatalf("doctor changed running state: before=%+v after=%+v", beforeStatus, afterStatus)
	}
	requireDoctorNoWrites(t, root, beforeFiles)
	for path, before := range map[string]os.FileInfo{fixture.socket: socketInfo, lockPath: lockInfo} {
		after, err := os.Lstat(path)
		if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
			t.Fatalf("runtime file changed: %s %v", path, err)
		}
	}
	controlCLIResult[control.QuitResult](t, fixture.env, "quit")
	stoppedRunProcess(t, process, fixture)
}

func TestCLIDoctorPrivacy(t *testing.T) {
	fixture := newRunFixture(t)
	seedDoctorCLI(t, fixture)
	root := doctorRoot(fixture)
	before := cliTree(t, root)
	runtime := filepath.Dir(filepath.Dir(fixture.socket))
	beforeRuntime := cliTree(t, runtime)
	for _, include := range []bool{false, true} {
		for _, jsonMode := range []bool{false, true} {
			args := []string{"doctor"}
			if include {
				args = append(args, "--include-bindings")
			}
			if jsonMode {
				args = append(args, "--json")
			}
			output := invokeCLI(t, fixture.env, nil, args...)
			checkCLIStatus(t, output, 1, true)
			for _, marker := range []string{"PRIVATE", "OTHER_LABEL", "Other profile"} {
				if strings.Contains(output.stdout, marker) {
					t.Fatalf("private output: %s", output.stdout)
				}
			}
			if strings.Contains(output.stdout, "SELECTED_LABEL") != include || strings.Contains(output.stdout, "KEY_U") != include || strings.ContainsRune(output.stdout, '\x1b') || strings.Contains(output.stdout, "SELECTED_LABEL\n") {
				t.Fatalf("disclosure=%v output=%q", include, output.stdout)
			}
			if jsonMode {
				report := parsedDoctorReport(t, output)
				if (report.ProfileDetails != nil) != include {
					t.Fatalf("details=%+v", report.ProfileDetails)
				}
				requireDoctorCode(t, report, diagnostics.WARN_CONTROL_NOT_RUNNING)
			}
		}
	}
	requireDoctorNoWrites(t, root, before)
	requireDoctorNoWrites(t, runtime, beforeRuntime)
}

func TestCLIDoctorMissingConfig(t *testing.T) {
	fixture := newRunFixture(t)
	if err := os.Remove(fixture.config); err != nil {
		t.Fatal(err)
	}
	root := doctorRoot(fixture)
	before := cliTree(t, root)
	for _, args := range [][]string{{"doctor"}, {"doctor", "--json"}} {
		output := invokeCLI(t, fixture.env, nil, args...)
		checkCLIStatus(t, output, 2, true)
		if len(args) == 2 {
			report := parsedDoctorReport(t, output)
			requireDoctorCode(t, report, config.ERR_CONFIG_NOT_FOUND)
			requireDoctorCode(t, report, diagnostics.WARN_CHECK_SKIPPED)
			requireDoctorCode(t, report, diagnostics.WARN_CONTROL_NOT_RUNNING)
		} else if !strings.Contains(output.stdout, config.ERR_CONFIG_NOT_FOUND) {
			t.Fatalf("text=%s", output.stdout)
		}
	}
	requireDoctorNoWrites(t, root, before)
	if _, err := os.Lstat(filepath.Dir(fixture.socket)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("runtime app created: %v", err)
	}
}

func TestCLIDoctorPermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("permission-denied fixture requires an unprivileged process")
	}
	fixture := newRunFixture(t)
	before := cliRead(t, fixture.config)
	if err := os.Chmod(fixture.config, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(fixture.config, 0600); err != nil {
			t.Error(err)
		}
	})
	for _, args := range [][]string{{"doctor"}, {"doctor", "--json"}} {
		output := invokeCLI(t, fixture.env, nil, args...)
		checkCLIStatus(t, output, 2, true)
		if len(args) == 2 {
			report := parsedDoctorReport(t, output)
			requireDoctorCode(t, report, config.ERR_CONFIG_INVALID)
			requireDoctorCode(t, report, diagnostics.ERR_DIAGNOSTIC_PERMISSION)
		} else if !strings.Contains(output.stdout, diagnostics.ERR_DIAGNOSTIC_PERMISSION) {
			t.Fatalf("text=%s", output.stdout)
		}
	}
	info, err := os.Stat(fixture.config)
	if err != nil || info.Mode() != 0 {
		t.Fatalf("config permission changed: %v", err)
	}
	if err := os.Chmod(fixture.config, 0600); err != nil {
		t.Fatal(err)
	}
	if string(cliRead(t, fixture.config)) != string(before) {
		t.Fatal("denied config content changed")
	}
	if _, err := os.Lstat(filepath.Dir(fixture.socket)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("runtime app created: %v", err)
	}
}

func TestCLIDoctorStale(t *testing.T) {
	fixture := newRunFixture(t)
	if err := os.Mkdir(filepath.Dir(fixture.socket), 0700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: fixture.socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	listener.SetUnlinkOnClose(false)
	if err := os.Chmod(fixture.socket, 0600); err != nil {
		listener.Close()
		t.Fatal(err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(fixture.socket)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"doctor"}, {"doctor", "--json"}} {
		output := invokeCLI(t, fixture.env, nil, args...)
		checkCLIStatus(t, output, 1, true)
		if len(args) == 2 {
			requireDoctorCode(t, parsedDoctorReport(t, output), diagnostics.WARN_CONTROL_STALE)
		} else if !strings.Contains(output.stdout, diagnostics.WARN_CONTROL_STALE) {
			t.Fatalf("text=%s", output.stdout)
		}
	}
	after, err := os.Lstat(fixture.socket)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
		t.Fatalf("stale socket changed: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(fixture.socket))
	if err != nil || len(entries) != 1 || entries[0].Name() != "control.sock" {
		t.Fatalf("runtime entries changed: %v %v", entries, err)
	}
}

func TestCLIDoctorHelp(t *testing.T) {
	fixture := newRunFixture(t)
	if err := os.Remove(fixture.config); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(fixture.config, 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"doctor", "--help"}, {"doctor", "--config=" + fixture.config, "--include-bindings", "--json", "--help"}} {
		output := invokeCLI(t, fixture.env, nil, args...)
		checkCLIStatus(t, output, 0, true)
		if !strings.HasPrefix(output.stdout, "Usage: azerlay doctor") || strings.Contains(output.stdout, "Doctor diagnostics") {
			t.Fatalf("help=%s", output.stdout)
		}
	}
	output := invokeCLI(t, fixture.env, nil, "doctor", "--config", fixture.config, "--unknown", "--json")
	checkCLIStatus(t, output, 2, true)
	requireDoctorCode(t, parsedDoctorReport(t, output), "ERR_CLI_USAGE")
	if _, err := os.Lstat(filepath.Dir(fixture.socket)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("help created runtime app: %v", err)
	}
}

func TestCLIDoctorMalformedResponse(t *testing.T) {
	fixture := newRunFixture(t)
	if err := os.Mkdir(filepath.Dir(fixture.socket), 0700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: fixture.socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := os.Chmod(fixture.socket, 0600); err != nil {
		t.Fatal(err)
	}
	if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
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
		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			done <- err
			return
		}
		_, err = control.ReadRequest(conn)
		if err == nil {
			_, err = conn.Write([]byte("{\n"))
		}
		done <- err
	}()
	output := invokeCLI(t, fixture.env, nil, "doctor", "--json")
	checkCLIStatus(t, output, 2, true)
	report := parsedDoctorReport(t, output)
	requireDoctorCode(t, report, control.ERR_CONTROL_REQUEST)
	for _, check := range report.Checks {
		if check.Code == diagnostics.WARN_CONTROL_STALE {
			t.Fatal("malformed live response was called stale")
		}
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("response peer did not stop")
	}
}
