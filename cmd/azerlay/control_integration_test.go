package main

import (
	"bytes"
	"context"
	"encoding/json"
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

const controlCLIBundle = `{"profiles":[{"id":"PRIVATE_ID_FIRST","name":"First profile\nline","inputs":[{"label":"PRIVATE_LABEL","types":["16","11","11"],"macro":"PRIVATE_MACRO","future":"PRIVATE_FIELD"}]},{"id":"PRIVATE_ID_SECOND","name":"Second profile","inputs":[]},{"id":"third","name":"duplicate","inputs":[]},{"id":"fourth","name":"duplicate","inputs":[]},{"id":"fifth","name":"-leading","inputs":[]}]}`

type liveControlCLI struct {
	env        []string
	manager    *config.Manager
	source     *profilesource.ImportedSource
	server     *control.Server
	configPath string
	dataHome   string
}

func startControlCLI(t *testing.T) *liveControlCLI {
	t.Helper()
	root := t.TempDir()
	runtime, err := os.MkdirTemp("", "az-cli-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(runtime); err != nil {
			t.Error(err)
		}
	})
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	env := cliEnvironment(root)
	for i, entry := range env {
		if strings.HasPrefix(entry, "XDG_RUNTIME_DIR=") {
			env[i] = "XDG_RUNTIME_DIR=" + runtime
		}
	}
	dataHome := filepath.Join(root, "XDG_DATA_HOME")
	source := profilesource.NewImportedSource(dataHome)
	prepared, err := profilesource.PrepareText(controlCLIBundle, profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.Import(t.Context(), prepared, 2, profilesource.Origin{Kind: "text"}); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "config.toml")
	cliWrite(t, configPath, []byte("schema_version = 1\n"), 0600)
	ctx, cancel := context.WithCancel(t.Context())
	manager, err := config.Start(ctx, configPath)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); waitControlCLIDone(t, manager.Done()) })
	controller, err := control.NewController(ctx, manager, source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(controller.Close)
	server, err := control.Start(ctx, controller)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})
	return &liveControlCLI{env, manager, source, server, configPath, dataHome}
}

func waitControlCLIDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("control resource did not stop")
	}
}

func awaitControlCLIConfig(t *testing.T, manager *config.Manager, predicate func(config.Snapshot) bool) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for !predicate(manager.Snapshot()) {
		select {
		case <-manager.Changes():
		case <-timer.C:
			t.Fatal("reload did not reach expected state")
		}
	}
}

func controlCLIResult[T any](t *testing.T, env []string, args ...string) T {
	t.Helper()
	output := invokeCLI(t, env, nil, append(args, "--json")...)
	checkCLIStatus(t, output, 0, true)
	for _, marker := range []string{"PRIVATE_ID", "PRIVATE_LABEL", "PRIVATE_MACRO", "PRIVATE_FIELD"} {
		if strings.Contains(output.stdout, marker) {
			t.Fatalf("private status metadata: %s", output.stdout)
		}
	}
	if args[0] == "status" {
		fields := parsedJSON(t, output.stdout).(map[string]any)["result"].(map[string]any)
		for _, field := range []string{"schema_version", "uptime_seconds", "visible", "active_profile", "device", "event_nodes", "event_rate", "dropped_count", "resync_count", "render_rate", "last_reload", "generation", "degraded_reasons"} {
			if _, ok := fields[field]; !ok {
				t.Fatalf("status missing %s", field)
			}
		}
	}
	var report struct {
		SchemaVersion int          `json:"schema_version"`
		Command       string       `json:"command"`
		OK            bool         `json:"ok"`
		Result        T            `json:"result"`
		Error         *reportError `json:"error"`
	}
	if err := json.Unmarshal([]byte(output.stdout), &report); err != nil {
		t.Fatal(err)
	}
	command := args[0]
	if command == "profiles" {
		command += " " + args[1]
	}
	if report.SchemaVersion != 1 || report.Command != command || !report.OK || report.Error != nil {
		t.Fatalf("invalid success: %s", output.stdout)
	}
	return report.Result
}

func TestCLIControlLive(t *testing.T) {
	live := startControlCLI(t)
	before := cliTree(t, live.dataHome)
	initial := controlCLIResult[control.Status](t, live.env, "status")
	if initial.Visible || initial.Generation != 1 || initial.ActiveProfile.ProfileIndex != 2 || initial.Device != nil || len(initial.DegradedReasons) != 3 {
		t.Fatalf("initial status: %+v", initial)
	}
	for _, step := range []struct {
		command string
		visible bool
	}{{"show", true}, {"hide", false}, {"toggle", true}} {
		result := controlCLIResult[control.VisibilityResult](t, live.env, step.command)
		if result.Visible != step.visible {
			t.Fatalf("%s visible=%t", step.command, result.Visible)
		}
	}
	selection := controlCLIResult[control.SelectionResult](t, live.env, "profiles", "select", "First profile\nline")
	if selection.ActiveProfile.ProfileIndex != 1 {
		t.Fatalf("selection: %+v", selection)
	}
	status := controlCLIResult[control.Status](t, live.env, "status")
	if status.ActiveProfile.ProfileIndex != 1 || !status.Visible || status.Generation != selection.Generation {
		t.Fatalf("active status: %+v", status)
	}
	saved := invokeCLI(t, live.env, nil, "profiles", "show", "--json")
	checkCLIStatus(t, saved, 0, true)
	var savedReport operationReport
	if err := json.Unmarshal([]byte(saved.stdout), &savedReport); err != nil {
		t.Fatal(err)
	}
	if *savedReport.Result.SelectedProfileIndex != 2 || !reflect.DeepEqual(before, cliTree(t, live.dataHome)) {
		t.Fatal("runtime selection changed saved state")
	}
	human := invokeCLI(t, live.env, nil, "status")
	checkCLIStatus(t, human, 0, true)
	if !strings.Contains(human.stdout, `"First profile\nline"`) || strings.Contains(human.stdout, "PRIVATE_") || strings.Contains(human.stdout, "First profile\nline") {
		t.Fatalf("unsafe text status: %q", human.stdout)
	}
	accepted := controlCLIResult[control.ReloadResult](t, live.env, "reload")
	if !accepted.Accepted || accepted.RequestGeneration <= initial.LastReload.RequestGeneration {
		t.Fatalf("reload acceptance: %+v", accepted)
	}
	awaitControlCLIConfig(t, live.manager, func(s config.Snapshot) bool { return s.Status.ConfigGeneration >= accepted.RequestGeneration })
	cliWrite(t, live.configPath, []byte("schema_version = 1\n[profile]\nselected_id = \"PRIVATE_ID_SECOND\"\n"), 0600)
	awaitControlCLIConfig(t, live.manager, func(s config.Snapshot) bool { return s.Config.Profile.SelectedID == "PRIVATE_ID_SECOND" })
	if current := controlCLIResult[control.Status](t, live.env, "status"); current.ActiveProfile.ProfileIndex != 1 {
		t.Fatal("reload replaced active selection")
	}
	terminated := invokeCLI(t, live.env, nil, "profiles", "select", "--json", "--", "-leading")
	checkCLIStatus(t, terminated, 0, true)
	if current := controlCLIResult[control.Status](t, live.env, "status"); current.ActiveProfile.ProfileIndex != 5 {
		t.Fatal("terminated selector was not preserved")
	}
	if result := controlCLIResult[control.QuitResult](t, live.env, "quit"); !result.Quitting {
		t.Fatal("quit not acknowledged")
	}
	waitControlCLIDone(t, live.server.Done())
	restarted, err := control.NewController(t.Context(), live.manager, live.source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restarted.Close)
	server, err := control.Start(t.Context(), restarted)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})
	if current := controlCLIResult[control.Status](t, live.env, "status"); current.ActiveProfile.ProfileIndex != 2 || current.Visible {
		t.Fatalf("session state persisted: %+v", current)
	}
}

func TestCLIControlFailures(t *testing.T) {
	live := startControlCLI(t)
	for _, item := range []struct{ selector, code string }{{"duplicate", "ERR_PROFILE_AMBIGUOUS"}, {"PRIVATE_MISSING_SELECTOR", "ERR_PROFILE_NOT_FOUND"}} {
		output := invokeCLI(t, live.env, nil, "profiles", "select", item.selector, "--json")
		checkCLIFailure(t, output, 1, "profiles select", item.code, "selection", "null")
		human := invokeCLI(t, live.env, nil, "profiles", "select", item.selector)
		if human.status != 1 || human.stdout != "" || !strings.Contains(human.stderr, item.code) || strings.Contains(human.stderr, item.selector) {
			t.Fatalf("unsafe text failure: %+v", human)
		}
		if strings.Contains(output.stdout, item.selector) {
			t.Fatalf("selector disclosed: %s", output.stdout)
		}
	}
	selected := controlCLIResult[control.SelectionResult](t, live.env, "profiles", "select", "PRIVATE_ID_FIRST")
	good := live.manager.Snapshot()
	cliWrite(t, live.configPath, []byte("PRIVATE_CONFIG = ["), 0600)
	accepted := controlCLIResult[control.ReloadResult](t, live.env, "reload")
	if !accepted.Accepted {
		t.Fatal("invalid reload was not accepted asynchronously")
	}
	awaitControlCLIConfig(t, live.manager, func(s config.Snapshot) bool { return s.Status.ConfigFailure.Code != "" })
	current := controlCLIResult[control.Status](t, live.env, "status")
	if current.LastReload.ConfigFailure == nil || current.LastReload.ConfigGeneration != good.Status.ConfigGeneration || current.ActiveProfile.ProfileIndex != selected.ActiveProfile.ProfileIndex || !reflect.DeepEqual(live.manager.Snapshot().Config, good.Config) {
		t.Fatalf("last-good state lost: %+v", current)
	}
	for _, jsonFlag := range [][]string{nil, {"--json"}} {
		output := invokeCLI(t, live.env, nil, append([]string{"status"}, jsonFlag...)...)
		if strings.Contains(output.stdout+output.stderr, "PRIVATE_CONFIG") || strings.Contains(output.stdout+output.stderr, "PRIVATE_ID") || strings.Contains(output.stdout+output.stderr, live.configPath) {
			t.Fatalf("private status: %+v", output)
		}
	}
	// A rejected output write must not replay the toggle or undo its transition.
	writer := &refusingWriter{}
	var stderr bytes.Buffer
	if code := run([]string{"toggle", "--json"}, &observedReader{}, writer, &stderr); code != 1 || writer.calls != 1 || stderr.Len() != 0 {
		t.Fatalf("failed write code=%d writes=%d", code, writer.calls)
	}
	if state := controlCLIResult[control.Status](t, live.env, "status"); !state.Visible || state.Generation != current.Generation+1 {
		t.Fatalf("mutation replayed or rolled back: %+v", state)
	}
	if err := live.server.Close(); err != nil {
		t.Fatal(err)
	}
	disconnected := invokeCLI(t, live.env, nil, "status", "--json")
	checkCLIFailure(t, disconnected, 1, "status", "ERR_CONTROL_UNAVAILABLE", "transport", "null")
}
