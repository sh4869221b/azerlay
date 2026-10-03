package control

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profilesource"
)

func TestProfileSnapshotImported(t *testing.T) {
	t.Parallel()
	for _, settings := range []string{"source='imported'\n", "source='imported'\nselected_id='first-id'\n"} {
		t.Run(settings, func(t *testing.T) {
			source := profilesource.NewImportedSource(t.TempDir())
			importControllerProfiles(t, source, controllerBundle)
			c, _, _ := startLocalController(t, settings, source)
			initial := c.ProfileSnapshot()
			want := "Second"
			if settings != "source='imported'\n" {
				want = "First"
			}
			if initial.Profile == nil || *initial.Profile.Name != want || initial.Source != (profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export"}) {
				t.Fatalf("wrong imported snapshot: %+v", initial)
			}
			for _, selector := range []string{"First", "Second", "First"} {
				if response := c.Dispatch(t.Context(), controllerRequest(MethodSelect, selector)); !response.OK {
					t.Fatalf("select failed: %+v", response)
				}
			}
			latest := c.ProfileSnapshot()
			if *latest.Profile.Name != "First" || latest.Source != initial.Source || latest.Generation <= initial.Generation || len(c.changes) != 1 || cap(c.changes) != 1 {
				t.Fatal("latest state or coalesced notification was lost")
			}
			<-c.Changes()
			before := controllerStatus(t, c).Generation
			c.Dispatch(t.Context(), controllerRequest(MethodSelect, "First"))
			if c.ProfileSnapshot() != latest || len(c.changes) != 0 || controllerStatus(t, c).Generation != before || *initial.Profile.Name != want {
				t.Fatal("same selection changed state or old snapshot")
			}
			c.Dispatch(t.Context(), controllerRequest(MethodShow, ""))
			if c.ProfileSnapshot() != latest {
				t.Fatal("visibility changed the profile generation")
			}
		})
	}
}

func awaitProfileChange(t *testing.T, c *Controller, predicate func() bool) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-c.Changes():
			if predicate() {
				return
			}
		case <-timer.C:
			t.Fatal("profile change notification did not converge")
		}
	}
}

func TestProfileSnapshotLocalWatchAndRestore(t *testing.T) {
	root, path := localControllerFixture(t)
	c, _, _ := startLocalController(t, localControllerSettings(root, "local", true), nil)
	initial := c.ProfileSnapshot()
	wantSource := profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-local-json"}
	if initial.Profile == nil || initial.Source != wantSource || initial.Selection.Source.Local.Root != root {
		t.Fatalf("local provenance missing: %+v", initial)
	}
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	awaitProfileChange(t, c, func() bool {
		return hasLocalDiagnostic(controllerStatus(t, c), profilesource.ERR_PROFILE_LOCAL_UNSUPPORTED)
	})
	if c.ProfileSnapshot() != initial {
		t.Fatal("malformed local update replaced last-good snapshot")
	}
	writeLocalControllerProfile(t, path, "Updated")
	awaitProfileChange(t, c, func() bool { return *c.ProfileSnapshot().Profile.Name == "Updated" })
	latest := c.ProfileSnapshot()
	if latest.Source != wantSource || latest.Generation <= initial.Generation || *initial.Profile.Name != "Local first" || hasLocalDiagnostic(controllerStatus(t, c), profilesource.ERR_PROFILE_LOCAL_UNSUPPORTED) {
		t.Fatal("local recovery changed old content or lost provenance")
	}
	c.Close()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	restored, _, _ := startLocalController(t, localControllerSettings(root, "local", false), nil)
	if snapshot := restored.ProfileSnapshot(); snapshot.Source != wantSource || snapshot.Profile == nil || *snapshot.Profile.Name != "Updated" {
		t.Fatalf("restored local provenance/content missing: %+v", snapshot)
	}
}

func TestProfileSnapshotAutoFallbackAndMissing(t *testing.T) {
	root, path := localControllerFixture(t)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	source := profilesource.NewImportedSource(home)
	selected := importControllerProfiles(t, source, `{"id":"fallback","name":"Fallback","inputs":[]}`)
	saved, _, _ := startLocalController(t, localControllerSettings(root, "auto", false), source)
	if snapshot := saved.ProfileSnapshot(); snapshot.Selection != selected || snapshot.Source.SourceScope != "azeron-software-export" || snapshot.Profile == nil || *snapshot.Profile.Name != "Fallback" {
		t.Fatalf("saved imported fallback provenance missing: %+v", snapshot)
	}
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
	data, err = json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	c, _, _ := startLocalController(t, localControllerSettings(root, "auto", false), source)
	snapshot := c.ProfileSnapshot()
	if snapshot.Selection != selected || snapshot.Profile == nil || *snapshot.Profile.Name != "Fallback" || snapshot.Source.SourceScope != "azeron-software-export" {
		t.Fatalf("auto imported fallback provenance missing: %+v", snapshot)
	}
	missing, _, _ := startLocalController(t, localControllerSettings(root, "local", false), nil)
	if snapshot := missing.ProfileSnapshot(); snapshot == nil || snapshot.Profile != nil || snapshot.Generation != 1 {
		t.Fatalf("missing profile state: %+v", snapshot)
	}
}

func TestProfileSnapshotOwnsNormalizedValues(t *testing.T) {
	name, release := "Owned", "on-release"
	id, pin, delay := 7, 12, 50
	value := profile.Profile{ID: &name, Name: &name, Raw: profile.RawProfileReference{Unknown: map[string]json.RawMessage{"private": json.RawMessage(`true`)}}, Controls: []profile.ControlBinding{{
		Label: &name, SourceIdentity: profile.SourceIdentity{InputID: &id, PinOne: &pin, PinTwo: &pin},
		Raw: profile.RawBindingReference{Fields: map[string]json.RawMessage{"private": json.RawMessage(`true`)}},
		Bindings: []profile.TriggerBinding{{Trigger: profile.TriggerLong, Kind: profile.BindingKeyboard,
			Actions:        []profile.Action{{Kind: profile.ActionKeyboard, Code: profile.KEY_W, Modifiers: []profile.CanonicalCode{profile.KEY_LEFTCTRL}}},
			Turbo:          &profile.TurboBinding{Code: profile.KEY_T, ClicksPerSecond: 10},
			Macro:          &profile.MacroBinding{RepeatWhileHeld: true, Steps: []profile.MacroStep{{Kind: profile.MacroStepButton, Code: profile.KEY_A, DurationMS: 25}}},
			Stick:          &profile.StickBinding{Mode: profile.StickModeKeyboard, KeyboardDirections: profile.KeyboardDirections{Up: profile.KEY_W}, AngleDegrees: 5},
			TriggerDelayMS: &delay, TriggerIntervalMS: &delay, ReleaseBehavior: &release,
			Unknown: &profile.UnknownBinding{Reason: "unsupported", RawDisplay: "Unknown"},
		}},
	}}}
	c := &Controller{active: &activeSelection{profile: value}}
	c.publishProfileLocked()
	before := c.ProfileSnapshot()
	if !reflect.DeepEqual(before.Profile.Raw, profile.RawProfileReference{}) || !reflect.DeepEqual(before.Profile.Controls[0].Raw, profile.RawBindingReference{}) {
		t.Fatal("opaque raw data entered render snapshot")
	}
	name, release, id, pin, delay = "Changed", "changed", 99, 99, 99
	binding := &value.Controls[0].Bindings[0]
	binding.Actions[0].Modifiers[0] = profile.KEY_S
	binding.Actions[0].Code = profile.KEY_D
	binding.Turbo.ClicksPerSecond = 99
	binding.Macro.Steps[0].DurationMS = 99
	binding.Stick.AngleDegrees = 99
	binding.Unknown.RawDisplay = "Changed"
	value.Controls[0].Bindings = nil
	c.publishProfileLocked()
	owned := before.Profile.Controls[0]
	b := owned.Bindings[0]
	if *before.Profile.Name != "Owned" || *before.Profile.ID != "Owned" || *owned.Label != "Owned" || *owned.SourceIdentity.InputID != 7 || *owned.SourceIdentity.PinOne != 12 || *owned.SourceIdentity.PinTwo != 12 ||
		b.Actions[0].Code != profile.KEY_W || b.Actions[0].Modifiers[0] != profile.KEY_LEFTCTRL || b.Turbo.ClicksPerSecond != 10 || b.Macro.Steps[0].DurationMS != 25 || !b.Macro.RepeatWhileHeld || b.Stick.AngleDegrees != 5 || b.Stick.KeyboardDirections.Up != profile.KEY_W || *b.TriggerDelayMS != 50 || *b.TriggerIntervalMS != 50 || *b.ReleaseBehavior != "on-release" || b.Unknown.RawDisplay != "Unknown" {
		t.Fatal("published snapshot retained aliases to mutable normalized data")
	}
	if latest := c.ProfileSnapshot(); *latest.Profile.Name != "Changed" || latest.Generation <= before.Generation || latest == before || latest != c.ProfileSnapshot() {
		t.Fatal("replacement snapshot did not publish owned latest data")
	}
}
