package control

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/profilesource"
)

const controllerLocalJSON = `{"id":"a","name":"Local first","version":1,"inputs":[],"isSoftware":true,"metaData":{"changedLogs":[{"softwareVersion":"2.0.2"}]}}`

func localControllerFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := filepath.Join(root, "Storage/DevicesStorage/device/ProfileStorage/profile_a.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(controllerLocalJSON), 0600); err != nil {
		t.Fatal(err)
	}
	return root, path
}

func localControllerSettings(root, policy string, watch bool) string {
	return fmt.Sprintf("source=%q\nlocal_store_path=%q\nlocal_device='device'\nlocal_profile_file='profile_a.json'\nwatch=%t\n", policy, root, watch)
}

func startLocalController(t *testing.T, settings string, source *profilesource.ImportedSource) (*Controller, *config.Manager, string) {
	t.Helper()
	manager, path := controllerManager(t, settings)
	c, err := NewController(t.Context(), manager, source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c, manager, path
}

func hasLocalDiagnostic(status Status, code string) bool {
	for _, reason := range status.DegradedReasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

func awaitLocalStatus(t *testing.T, c *Controller, predicate func(Status) bool) Status {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		status := controllerStatus(t, c)
		if predicate(status) {
			return status
		}
		select {
		case <-tick.C:
		case <-timer.C:
			t.Fatalf("local status did not converge: %+v", status)
		}
	}
}

func writeLocalControllerProfile(t *testing.T, path, name string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Replace(controllerLocalJSON, "Local first", name, 1)), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLocalControllerWatchAndRestart(t *testing.T) {
	root, path := localControllerFixture(t)
	c, manager, configPath := startLocalController(t, localControllerSettings(root, "local", true), nil)
	status := controllerStatus(t, c)
	if status.ActiveProfile == nil || status.ActiveProfile.Source != "local" || status.ActiveProfile.SourceRef != "Storage/DevicesStorage/device/ProfileStorage/profile_a.json" || status.ActiveProfile.ProfileIndex != 1 || *status.ActiveProfile.Name != "Local first" {
		t.Fatalf("local startup: %+v", status)
	}
	var wire bytes.Buffer
	request := controllerRequest(MethodStatus, "")
	if err := json.NewEncoder(&wire).Encode(c.Dispatch(t.Context(), request)); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadResponse(&wire, request); err != nil {
		t.Fatalf("local client decode: %v", err)
	}
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	status = awaitLocalStatus(t, c, func(s Status) bool { return hasLocalDiagnostic(s, profilesource.ERR_PROFILE_LOCAL_UNSUPPORTED) })
	if *status.ActiveProfile.Name != "Local first" {
		t.Fatal("partial definition replaced last-good")
	}
	temp := filepath.Join(filepath.Dir(path), "replacement.tmp")
	writeLocalControllerProfile(t, temp, "Local repaired")
	if err := os.Rename(temp, path); err != nil {
		t.Fatal(err)
	}
	status = awaitLocalStatus(t, c, func(s Status) bool {
		return *s.ActiveProfile.Name == "Local repaired" && !hasLocalDiagnostic(s, profilesource.ERR_PROFILE_LOCAL_UNSUPPORTED)
	})
	statePath := filepath.Join(os.Getenv("XDG_STATE_HOME"), "azerlay/last-good.json")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(statePath, 0400); err != nil {
		t.Fatal(err)
	}
	writeLocalControllerProfile(t, path, "Local state failure")
	status = awaitLocalStatus(t, c, func(s Status) bool { return hasLocalDiagnostic(s, profilesource.ERR_PROFILE_LOCAL_STATE) })
	if *status.ActiveProfile.Name != "Local repaired" {
		t.Fatal("state publication failure replaced active definition")
	}
	after, err := os.ReadFile(statePath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("failed publication changed saved bytes: %v", err)
	}
	if err := os.Chmod(statePath, 0600); err != nil {
		t.Fatal(err)
	}
	writeControllerConfig(t, configPath, "source='imported'\n")
	awaitControllerConfig(t, manager, func(s config.Snapshot) bool { return s.Config.Profile.Source == "imported" })
	writeLocalControllerProfile(t, path, "Local policy retained")
	awaitLocalStatus(t, c, func(s Status) bool { return *s.ActiveProfile.Name == "Local policy retained" })
	c.Close()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	restarted, _, _ := startLocalController(t, localControllerSettings(root, "local", false), nil)
	status = controllerStatus(t, restarted)
	if status.ActiveProfile == nil || *status.ActiveProfile.Name != "Local policy retained" || !hasLocalDiagnostic(status, profilesource.ERR_PROFILE_LOCAL_READ) {
		t.Fatalf("restart last-good: %+v", status)
	}
	if restarted.localCancel != nil {
		t.Fatal("watch=false started watcher")
	}
}

func TestLocalControllerPriority(t *testing.T) {
	for _, policy := range []string{"auto", "local"} {
		t.Run(policy, func(t *testing.T) {
			root, path := localControllerFixture(t)
			source := profilesource.NewImportedSource(t.TempDir())
			importControllerProfiles(t, source, controllerBundle)
			if err := os.WriteFile(path, []byte(strings.Replace(controllerLocalJSON, "2.0.2", "9.9.9", 1)), 0600); err != nil {
				t.Fatal(err)
			}
			c, _, _ := startLocalController(t, localControllerSettings(root, policy, false), source)
			status := controllerStatus(t, c)
			if !hasLocalDiagnostic(status, profilesource.ERR_PROFILE_LOCAL_UNSUPPORTED) {
				t.Fatalf("missing safe local failure: %+v", status)
			}
			if policy == "local" {
				if status.ActiveProfile != nil || c.Dispatch(t.Context(), controllerRequest(MethodSelect, "First")).OK {
					t.Fatal("explicit local silently fell back")
				}
			} else if status.ActiveProfile == nil || status.ActiveProfile.Source != "imported" || status.ActiveProfile.ProfileIndex != 2 {
				t.Fatalf("auto fallback lost saved ordinal: %+v", status)
			}
			wire, err := json.Marshal(status)
			if err != nil || strings.Contains(string(wire), root) || strings.Contains(string(wire), "9.9.9") {
				t.Fatalf("unsafe diagnostics: %s %v", wire, err)
			}
		})
	}
}

func TestAutoLocalMissingAndAmbiguousRoots(t *testing.T) {
	_, _ = localControllerFixture(t)
	root := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "Azeron Software")
	path := filepath.Join(root, "Storage/DevicesStorage/device/ProfileStorage/profile_a.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	writeLocalControllerProfile(t, path, "Local first")
	settings := localControllerSettings("", "auto", true)
	c, _, _ := startLocalController(t, settings, profilesource.NewImportedSource(t.TempDir()))
	c.Close()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	restarted, _, _ := startLocalController(t, settings, profilesource.NewImportedSource(t.TempDir()))
	if status := controllerStatus(t, restarted); status.ActiveProfile == nil || status.ActiveProfile.Source != "local" || !hasLocalDiagnostic(status, profilesource.ERR_PROFILE_LOCAL_READ) {
		t.Fatalf("missing auto source lost last-good: %+v", status)
	}
	writeLocalControllerProfile(t, path, "Auto recreated")
	awaitLocalStatus(t, restarted, func(s Status) bool {
		return *s.ActiveProfile.Name == "Auto recreated" && !hasLocalDiagnostic(s, profilesource.ERR_PROFILE_LOCAL_READ)
	})
	restarted.Close()
	other := filepath.Join(os.Getenv("HOME"), ".config/Azeron Software/Storage/DevicesStorage/device/ProfileStorage/profile_a.json")
	if err := os.MkdirAll(filepath.Dir(other), 0700); err != nil {
		t.Fatal(err)
	}
	writeLocalControllerProfile(t, other, "Ambiguous new root")
	ambiguous, _, _ := startLocalController(t, settings, profilesource.NewImportedSource(t.TempDir()))
	status := controllerStatus(t, ambiguous)
	if status.ActiveProfile == nil || *status.ActiveProfile.Name != "Auto recreated" || !hasLocalDiagnostic(status, profilesource.ERR_PROFILE_LOCAL_READ) || ambiguous.localCancel != nil {
		t.Fatalf("ambiguous roots adopted another source: %+v", status)
	}
}

func TestLocalSessionSelectStopsAndJoins(t *testing.T) {
	root, path := localControllerFixture(t)
	source := profilesource.NewImportedSource(t.TempDir())
	importControllerProfiles(t, source, controllerBundle)
	c, _, _ := startLocalController(t, localControllerSettings(root, "auto", true), source)
	if response := c.Dispatch(t.Context(), controllerRequest(MethodSelect, "missing")); response.OK {
		t.Fatal("missing imported selector succeeded")
	}
	writeLocalControllerProfile(t, path, "Failed select kept watch")
	awaitLocalStatus(t, c, func(s Status) bool { return *s.ActiveProfile.Name == "Failed select kept watch" })
	beforeState, err := os.ReadFile(filepath.Join(os.Getenv("XDG_STATE_HOME"), "azerlay/last-good.json"))
	if err != nil {
		t.Fatal(err)
	}
	writeLocalControllerProfile(t, path, "Pending local reload")
	if response := c.Dispatch(t.Context(), controllerRequest(MethodSelect, "First")); !response.OK || response.Result.(SelectionResult).ActiveProfile.Source != "imported" {
		t.Fatalf("session imported selection: %+v", response)
	}
	select {
	case <-c.localDone:
	default:
		t.Fatal("select returned before watcher/read joined")
	}
	afterState, err := os.ReadFile(filepath.Join(os.Getenv("XDG_STATE_HOME"), "azerlay/last-good.json"))
	if err != nil || !bytes.Equal(beforeState, afterState) {
		t.Fatalf("session selection changed last-good: %v", err)
	}
	writeLocalControllerProfile(t, path, "After session imported")
	if status := controllerStatus(t, c); status.ActiveProfile.Source != "imported" || *status.ActiveProfile.Name != "First" {
		t.Fatalf("stale local publication replaced session: %+v", status)
	}
}

func TestLocalSessionSelectCompletesAfterRequestCancellationDuringJoin(t *testing.T) {
	root, _ := localControllerFixture(t)
	source := profilesource.NewImportedSource(t.TempDir())
	importControllerProfiles(t, source, controllerBundle)
	c, _, _ := startLocalController(t, localControllerSettings(root, "auto", true), source)
	ctx, cancelRequest := context.WithCancel(t.Context())
	defer cancelRequest()
	stopLocal := c.localCancel
	c.localCancel = func() {
		stopLocal()
		cancelRequest()
	}
	response := c.Dispatch(ctx, controllerRequest(MethodSelect, "First"))
	if ctx.Err() != context.Canceled {
		t.Fatal("request cancellation did not occur during local shutdown")
	}
	if !response.OK {
		t.Fatalf("accepted selection failed after stopping its watcher: %+v", response.Error)
	}
	status := controllerStatus(t, c)
	if status.ActiveProfile.Source != "imported" || *status.ActiveProfile.Name != "First" {
		t.Fatalf("accepted imported selection did not finish: %+v", status.ActiveProfile)
	}
	select {
	case <-c.localDone:
	default:
		t.Fatal("accepted selection returned before local work joined")
	}
}

func TestLocalWatchFailurePersists(t *testing.T) {
	root, path := localControllerFixture(t)
	c, _, _ := startLocalController(t, localControllerSettings(root, "local", true), nil)
	if err := os.Rename(filepath.Dir(path), filepath.Dir(path)+"-moved"); err != nil {
		t.Fatal(err)
	}
	status := awaitLocalStatus(t, c, func(s Status) bool { return hasLocalDiagnostic(s, profilesource.ERR_PROFILE_LOCAL_WATCH) })
	if *status.ActiveProfile.Name != "Local first" {
		t.Fatal("stopped watcher discarded last-good")
	}
	c.Close()
}

func TestLocalConfiguredResolutionIsReadOnly(t *testing.T) {
	root, path := localControllerFixture(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	selection, p, err := ResolveConfiguredProfile(t.Context(), nil, config.Profile{Source: "local", LocalStorePath: root, LocalDevice: "device", LocalProfileFile: "profile_a.json", Watch: true})
	if err != nil || selection.Source.Local.Root != root || p.Name == nil || *p.Name != "Local first" {
		t.Fatalf("readonly local resolution: %+v %+v %v", selection, p, err)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("XDG_STATE_HOME"), "azerlay")); !os.IsNotExist(err) {
		t.Fatalf("diagnosis created state: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("diagnosis changed source: %v", err)
	}
}

func TestLocalConsumerRejectsCanceledCandidate(t *testing.T) {
	root, _ := localControllerFixture(t)
	ref := profilesource.LocalRef{Root: root, Device: "device", File: "profile_a.json"}
	candidate, err := profilesource.NewAzeronLocalSource(root).LoadCandidate(t.Context(), profilesource.SourceRef{Local: ref})
	if err != nil {
		t.Fatal(err)
	}
	source := profilesource.NewImportedSource(t.TempDir())
	importControllerProfiles(t, source, controllerBundle)
	c, _, _ := startLocalController(t, "source='auto'\n", source)
	ctx, cancel := context.WithCancel(t.Context())
	changes := make(chan profilesource.SourceChange)
	done := make(chan struct{})
	c.localEnabled, c.localCancel, c.localDone = true, cancel, done
	go c.consumeLocal(ctx, profilesource.LocalStateSelection{Policy: "auto", StorePath: root, Device: "device", File: "profile_a.json"}, changes, done)
	go func() {
		<-ctx.Done()
		changes <- profilesource.SourceChange{Ref: profilesource.SourceRef{Local: ref}, Candidate: &candidate}
		close(changes)
	}()
	if response := c.Dispatch(t.Context(), controllerRequest(MethodSelect, "First")); !response.OK {
		t.Fatalf("select while local result paused: %+v", response)
	}
	if c.active.selection.Source.Local != (profilesource.LocalRef{}) || *c.active.profile.Name != "First" {
		t.Fatal("canceled queued candidate replaced imported selection")
	}
	if snapshot := c.ProfileSnapshot(); snapshot.Selection.Source.Local != (profilesource.LocalRef{}) || snapshot.Source.SourceScope != "azeron-software-export" || *snapshot.Profile.Name != "First" {
		t.Fatal("canceled queued candidate replaced published render state")
	}
	select {
	case <-done:
	default:
		t.Fatal("session selection did not join paused result producer")
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("XDG_STATE_HOME"), "azerlay")); !os.IsNotExist(err) {
		t.Fatalf("canceled queued candidate saved: %v", err)
	}
}

func TestAutoImportedFallbackSingleOnly(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	source := profilesource.NewImportedSource(home)
	first := importControllerProfiles(t, source, `{"id":"first","name":"First single","inputs":[]}`)
	second := importControllerProfiles(t, source, `{"id":"second","name":"Second single","inputs":[]}`)
	importControllerProfiles(t, source, controllerBundle)
	indexPath := filepath.Join(home, "azerlay/cache/source-index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	var index map[string]json.RawMessage
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatal(err)
	}
	index["selected"] = json.RawMessage("null")
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(index["sources"], &entries); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		entry["imported_at"] = json.RawMessage(`"2026-10-03T00:00:00Z"`)
	}
	index["sources"], err = json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	selected, p, err := ResolveConfiguredProfile(t.Context(), source, config.Profile{Source: "auto"})
	if err != nil || selected.Source != second.Source || selected.ProfileIndex != 1 || *p.Name != "Second single" {
		t.Fatalf("equal-time later single fallback: %+v %+v %v", selected, p, err)
	}
	if err := os.WriteFile(filepath.Join(home, "azerlay/sources", second.Source.Hash+".azeron"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	selected, _, err = ResolveConfiguredProfile(t.Context(), source, config.Profile{Source: "auto"})
	if err != nil || selected.Source != first.Source {
		t.Fatalf("unloadable newest fallback: %+v %v", selected, err)
	}
	if _, _, err := ResolveConfiguredProfile(t.Context(), source, config.Profile{Source: "imported"}); err == nil || err.Error() != profilesource.ERR_PROFILE_NOT_FOUND {
		t.Fatalf("explicit imported chose unsaved ordinal: %v", err)
	}
	if _, _, err := ResolveConfiguredProfile(t.Context(), source, config.Profile{Source: "auto", SelectedID: "missing"}); err == nil || err.Error() != profilesource.ERR_PROFILE_STORAGE {
		t.Fatalf("explicit ID fell back: %v", err)
	}
	if err := os.WriteFile(indexPath, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ResolveConfiguredProfile(t.Context(), source, config.Profile{Source: "auto"}); err == nil || err.Error() != profilesource.ERR_PROFILE_STORAGE {
		t.Fatalf("corrupt index scanned orphan sources: %v", err)
	}
}

func TestLocalInitialStateFailureFallback(t *testing.T) {
	for _, policy := range []string{"auto", "local"} {
		t.Run(policy, func(t *testing.T) {
			root, _ := localControllerFixture(t)
			stateHome := os.Getenv("XDG_STATE_HOME")
			if err := os.Chmod(stateHome, 0500); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(stateHome, 0700); err != nil {
					t.Error(err)
				}
			})
			source := profilesource.NewImportedSource(t.TempDir())
			importControllerProfiles(t, source, controllerBundle)
			c, _, _ := startLocalController(t, localControllerSettings(root, policy, false), source)
			status := controllerStatus(t, c)
			if !hasLocalDiagnostic(status, profilesource.ERR_PROFILE_LOCAL_STATE) {
				t.Fatalf("initial persistence failure hidden: %+v", status)
			}
			if policy == "local" && status.ActiveProfile != nil || policy == "auto" && (status.ActiveProfile == nil || status.ActiveProfile.Source != "imported") {
				t.Fatalf("state failure priority: %+v", status)
			}
		})
	}
}

func TestLocalProtocolRejectsUnsafeRefs(t *testing.T) {
	t.Parallel()
	for _, ref := range []string{"/absolute/store", "Storage/DevicesStorage/../ProfileStorage/profile_a.json", "Storage/DevicesStorage/device/ProfileStorage/../profile_a.json", "Storage/DevicesStorage/device/ProfileStorage/a\\b", "Storage/DevicesStorage/device/ProfileStorage/"} {
		encoded, err := json.Marshal(ProfileStatus{Source: "local", SourceRef: ref, ProfileIndex: 1})
		if err != nil {
			t.Fatal(err)
		}
		if err := validateProfile(encoded); err == nil {
			t.Fatalf("unsafe wire ref accepted: %q", ref)
		}
	}
	encoded, err := json.Marshal(ProfileStatus{Source: "local", SourceRef: "Storage/DevicesStorage/device/ProfileStorage/profile_a.json", ProfileIndex: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateProfile(encoded); err == nil {
		t.Fatal("local ordinal greater than one accepted")
	}
}

func TestAutoLocalAndImportedFailuresRemainVisible(t *testing.T) {
	root, path := localControllerFixture(t)
	sourceHome := t.TempDir()
	source := profilesource.NewImportedSource(sourceHome)
	importControllerProfiles(t, source, controllerBundle)
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceHome, "azerlay/cache/source-index.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	c, _, _ := startLocalController(t, localControllerSettings(root, "auto", false), source)
	status := controllerStatus(t, c)
	if status.ActiveProfile != nil || !hasLocalDiagnostic(status, profilesource.ERR_PROFILE_LOCAL_UNSUPPORTED) || !hasLocalDiagnostic(status, profilesource.ERR_PROFILE_STORAGE) {
		t.Fatalf("one failed source hid the other: %+v", status)
	}
}
