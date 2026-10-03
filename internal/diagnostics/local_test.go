package diagnostics

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/profilesource"
)

func TestLocalProfileDiagnosisRetainsDegradedAvailability(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	root := t.TempDir()
	path := filepath.Join(root, "Storage/DevicesStorage/device/ProfileStorage/profile_a.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"id":"a","name":"Synthetic","version":1,"inputs":[],"isSoftware":true,"metaData":{"changedLogs":[{"softwareVersion":"2.0.2"}]}}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	settings := config.Profile{Source: "local", LocalStorePath: root, LocalDevice: "device", LocalProfileFile: "profile_a.json", Watch: true}
	checks, _ := collectProfile(t.Context(), settings, false)
	if len(checks) != 1 || checks[0].Code != OK_PROFILE_SOURCE {
		t.Fatalf("supported local diagnosis: %+v", checks)
	}
	if _, err := os.Stat(filepath.Join(stateHome, "azerlay")); !os.IsNotExist(err) {
		t.Fatalf("diagnosis created state: %v", err)
	}
	t.Setenv("HOME", "")
	t.Setenv("XDG_DATA_HOME", "relative")
	auto := settings
	auto.Source = "auto"
	checks, _ = collectProfile(t.Context(), auto, false)
	if len(checks) != 1 || checks[0].Code != OK_PROFILE_SOURCE {
		t.Fatalf("import placement blocked local-first diagnosis: %+v", checks)
	}
	ref := profilesource.LocalRef{Root: root, Device: "device", File: "profile_a.json"}
	candidate, err := profilesource.NewAzeronLocalSource(root).LoadCandidate(t.Context(), profilesource.SourceRef{Local: ref})
	if err != nil {
		t.Fatal(err)
	}
	if err := profilesource.SaveLocalState(t.Context(), profilesource.LocalStateSelection{Policy: "local", StorePath: root, Device: "device", File: "profile_a.json"}, ref, candidate); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(stateHome, "azerlay/last-good.json")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	checks, details := collectProfile(t.Context(), settings, true)
	if len(checks) != 2 || checks[0].Code != profilesource.ERR_PROFILE_LOCAL_READ || checks[1].Code != OK_PROFILE_SOURCE || details == nil || details.ProfileIndex != 1 {
		t.Fatalf("last-good diagnosis: %+v %+v", checks, details)
	}
	after, err := os.ReadFile(statePath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("diagnosis mutated last-good: %v", err)
	}
}
