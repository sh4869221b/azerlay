package control

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profilesource"
)

const controllerBundle = `{"profiles":[{"id":"first-id","name":"First","inputs":[]},{"id":"second-id","name":"Second","inputs":[]}]}`

func controllerManager(t *testing.T, settings string) (*config.Manager, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	writeControllerConfig(t, path, settings)
	ctx, cancel := context.WithCancel(t.Context())
	m, err := config.Start(ctx, path)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		select {
		case <-m.Done():
		case <-time.After(5 * time.Second):
			t.Error("manager did not stop")
		}
	})
	return m, path
}

func writeControllerConfig(t *testing.T, path, settings string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("schema_version = 1\n[profile]\n"+settings), 0600); err != nil {
		t.Fatal(err)
	}
}

func importControllerProfiles(t *testing.T, source *profilesource.ImportedSource, input string) profilesource.Selection {
	t.Helper()
	prepared, err := profilesource.PrepareText(input, profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export"})
	if err != nil {
		t.Fatal(err)
	}
	selection, err := source.Import(t.Context(), prepared, len(prepared.Bundle().Profiles), profilesource.Origin{Kind: "text"})
	if err != nil {
		t.Fatal(err)
	}
	return selection
}

func controllerRequest(method Method, selector string) Request {
	return Request{Version: 1, ID: "test", Method: method, Params: Params{Selector: selector}}
}

func controllerStatus(t *testing.T, c *Controller) Status {
	t.Helper()
	response := c.Dispatch(t.Context(), controllerRequest(MethodStatus, ""))
	if !response.OK {
		t.Fatalf("status failed: %+v", response.Error)
	}
	return response.Result.(Status)
}

func awaitControllerConfig(t *testing.T, m *config.Manager, predicate func(config.Snapshot) bool) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for !predicate(m.Snapshot()) {
		select {
		case <-m.Changes():
		case <-deadline.C:
			t.Fatal("configuration did not reach expected state")
		}
	}
}

func TestControlController(t *testing.T) {
	t.Parallel()
	m, path := controllerManager(t, "")
	source := profilesource.NewImportedSource(t.TempDir())
	saved := importControllerProfiles(t, source, controllerBundle)
	c, err := NewController(t.Context(), m, source)
	if err != nil {
		t.Fatal(err)
	}
	s := controllerStatus(t, c)
	if s.Generation != 1 || s.Visible || s.UptimeSeconds < 0 || s.ActiveProfile.ProfileIndex != 2 || s.ActiveProfile.SourceRef != saved.Source.Hash {
		t.Fatalf("initial status: %+v", s)
	}
	if s.Device != nil || s.EventNodes == nil || len(s.EventNodes) != 0 || s.EventRate != nil || s.DroppedCount != nil || s.ResyncCount != nil || s.RenderRate != nil || len(s.DegradedReasons) != 3 {
		t.Fatalf("unavailable status: %+v", s)
	}
	for _, step := range []struct {
		method     Method
		visible    bool
		generation uint64
	}{
		{MethodHide, false, 1}, {MethodShow, true, 2}, {MethodShow, true, 2}, {MethodToggle, false, 3}, {MethodToggle, true, 4}, {MethodHide, false, 5},
	} {
		response := c.Dispatch(t.Context(), controllerRequest(step.method, ""))
		if !response.OK || response.Result.(VisibilityResult).Visible != step.visible || controllerStatus(t, c).Generation != step.generation {
			t.Fatalf("transition %s: %+v", step.method, response)
		}
	}
	response := c.Dispatch(t.Context(), controllerRequest(MethodSelect, "First"))
	if !response.OK || response.Result.(SelectionResult).Generation != 6 {
		t.Fatalf("select: %+v", response)
	}
	c.Dispatch(t.Context(), controllerRequest(MethodSelect, "first-id"))
	if controllerStatus(t, c).Generation != 6 {
		t.Fatal("reselect incremented generation")
	}
	response = c.Dispatch(t.Context(), controllerRequest(MethodReload, ""))
	if !response.OK || !response.Result.(ReloadResult).Accepted || response.Result.(ReloadResult).RequestGeneration < 2 {
		t.Fatalf("reload: %+v", response)
	}
	accepted := response.Result.(ReloadResult).RequestGeneration
	awaitControllerConfig(t, m, func(s config.Snapshot) bool { return s.Status.ConfigGeneration >= accepted })
	writeControllerConfig(t, path, "source = \"local\"\n")
	awaitControllerConfig(t, m, func(s config.Snapshot) bool { return s.Config.Profile.Source == "local" })
	s = controllerStatus(t, c)
	if s.ActiveProfile.ProfileIndex != 1 || s.Generation != 6 {
		t.Fatalf("reload replaced active selection: %+v", s)
	}
	response = c.Dispatch(t.Context(), controllerRequest(MethodSelect, "second-id"))
	if !response.OK {
		t.Fatalf("reload changed source policy: %+v", response)
	}
	if err := os.WriteFile(path, []byte("private-invalid = ["), 0600); err != nil {
		t.Fatal(err)
	}
	awaitControllerConfig(t, m, func(s config.Snapshot) bool { return s.Status.ConfigFailure.Code != "" })
	s = controllerStatus(t, c)
	if s.LastReload.ConfigFailure == nil || len(s.DegradedReasons) != 4 || s.Generation != 7 {
		t.Fatalf("reload failure: %+v", s)
	}
	encoded, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "private-invalid") || strings.Contains(string(encoded), "first-id") || strings.Contains(string(encoded), path) {
		t.Fatalf("unsafe status: %s", encoded)
	}
	response = c.Dispatch(t.Context(), controllerRequest(MethodQuit, ""))
	if !response.OK || !response.Result.(QuitResult).Quitting || controllerStatus(t, c).Generation != 7 {
		t.Fatalf("quit intent: %+v", response)
	}
}

func TestControlOverlayObservation(t *testing.T) {
	t.Parallel()
	m, _ := controllerManager(t, "")
	source := profilesource.NewImportedSource(t.TempDir())
	importControllerProfiles(t, source, controllerBundle)
	c, err := NewController(t.Context(), m, source)
	if err != nil {
		t.Fatal(err)
	}
	if controllerStatus(t, c).Overlay != nil {
		t.Fatal("unattached backend reported an observation")
	}
	if response := c.Dispatch(t.Context(), controllerRequest(MethodShow, "")); !response.OK {
		t.Fatal(response.Error)
	}
	diagnostic := Diagnostic{Code: "ERR_OVERLAY_INPUT_REGION", Stage: "overlay", Reason: "Input region is unavailable."}
	c.SetOverlayStatus(OverlayStatus{Mapped: false, InputRegionApplied: false}, &diagnostic)
	diagnostic.Reason = "changed by caller"
	status := controllerStatus(t, c)
	if !status.Visible || status.Overlay == nil || status.Overlay.Mapped || status.Overlay.InputRegionApplied || status.Generation != 2 {
		t.Fatalf("requested and observed state: %+v", status)
	}
	if got := status.DegradedReasons[len(status.DegradedReasons)-1]; got.Reason != "Input region is unavailable." || got.Stage != "overlay" {
		t.Fatalf("diagnostic was not copied: %+v", got)
	}
	status.Overlay.Mapped = true
	c.SetOverlayStatus(OverlayStatus{Mapped: true, InputRegionApplied: true}, nil)
	status = controllerStatus(t, c)
	if !status.Overlay.Mapped || !status.Overlay.InputRegionApplied || len(status.DegradedReasons) != 3 || status.Generation != 2 {
		t.Fatalf("observation update changed request state: %+v", status)
	}
	status.Overlay.Mapped = false
	if !controllerStatus(t, c).Overlay.Mapped {
		t.Fatal("status caller mutated retained observation")
	}
}

func TestControlVisibilityNotification(t *testing.T) {
	t.Parallel()
	m, _ := controllerManager(t, "")
	c, err := NewController(t.Context(), m, profilesource.NewImportedSource(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []Method{MethodShow, MethodShow} {
		if response := c.Dispatch(t.Context(), controllerRequest(method, "")); !response.OK {
			t.Fatal(response.Error)
		}
		select {
		case <-c.VisibilityChanges():
		default:
			t.Fatalf("accepted %s did not wake the consumer", method)
		}
	}
	if !c.RequestedVisible() || controllerStatus(t, c).Generation != 2 {
		t.Fatal("idempotent show changed request generation")
	}
	var workers sync.WaitGroup
	for range 20 {
		workers.Go(func() {
			c.Dispatch(t.Context(), controllerRequest(MethodToggle, ""))
		})
	}
	workers.Wait()
	c.Dispatch(t.Context(), controllerRequest(MethodHide, ""))
	select {
	case <-c.VisibilityChanges():
	default:
		t.Fatal("concurrent requests did not leave a notification")
	}
	if c.RequestedVisible() || controllerStatus(t, c).Visible {
		t.Fatal("consumer did not observe latest requested visibility")
	}
	select {
	case <-c.VisibilityChanges():
		t.Fatal("coalesced channel held multiple notifications")
	default:
	}
}

func TestControlSelection(t *testing.T) {
	t.Parallel()
	t.Run("startup policy", func(t *testing.T) {
		source := profilesource.NewImportedSource(t.TempDir())
		importControllerProfiles(t, source, controllerBundle)
		for _, tc := range []struct {
			settings string
			index    int
			code     string
		}{
			{"selected_id = \"first-id\"", 1, ""}, {"selected_id = \"First\"", 0, profilesource.ERR_PROFILE_NOT_FOUND},
			{"source = \"local\"", 0, ERR_PROFILE_SOURCE_UNAVAILABLE},
		} {
			m, _ := controllerManager(t, tc.settings)
			c, err := NewController(t.Context(), m, source)
			if err != nil {
				t.Fatal(err)
			}
			s := controllerStatus(t, c)
			if tc.code == "" {
				if s.ActiveProfile == nil || s.ActiveProfile.ProfileIndex != tc.index {
					t.Fatalf("configured startup: %+v", s)
				}
			} else {
				if s.ActiveProfile != nil || s.DegradedReasons[3].Code != tc.code {
					t.Fatalf("degraded startup: %+v", s)
				}
				if tc.code == ERR_PROFILE_SOURCE_UNAVAILABLE && c.Dispatch(t.Context(), controllerRequest(MethodSelect, "First")).Error.Code != tc.code {
					t.Fatal("local source allowed selection")
				}
			}
		}
	})
	t.Run("empty and cancellation", func(t *testing.T) {
		m, _ := controllerManager(t, "")
		source := profilesource.NewImportedSource(t.TempDir())
		c, err := NewController(t.Context(), m, source)
		if err != nil {
			t.Fatal(err)
		}
		if s := controllerStatus(t, c); s.ActiveProfile != nil || s.DegradedReasons[3].Code != profilesource.ERR_PROFILE_NOT_FOUND {
			t.Fatalf("empty: %+v", s)
		}
		importControllerProfiles(t, source, controllerBundle)
		if response := c.Dispatch(t.Context(), controllerRequest(MethodSelect, "First")); !response.OK {
			t.Fatalf("recovery: %+v", response)
		}
		if len(controllerStatus(t, c).DegradedReasons) != 3 {
			t.Fatal("selection failure not cleared")
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := NewController(ctx, m, source); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled initialization: %v", err)
		}
		before := controllerStatus(t, c)
		for _, method := range []Method{MethodSelect, MethodShow, MethodToggle, MethodReload, MethodQuit} {
			if response := c.Dispatch(ctx, controllerRequest(method, "Second")); response.OK || response.Error.Code != ERR_CONTROL_UNAVAILABLE {
				t.Fatalf("cancelled dispatch: %+v", response)
			}
		}
		after := controllerStatus(t, c)
		if before.Generation != after.Generation || !reflect.DeepEqual(before.ActiveProfile, after.ActiveProfile) {
			t.Fatal("cancelled operation changed state")
		}
	})
	t.Run("uniqueness and storage", func(t *testing.T) {
		m, _ := controllerManager(t, "")
		home := t.TempDir()
		source := profilesource.NewImportedSource(home)
		first := importControllerProfiles(t, source, controllerBundle)
		second := importControllerProfiles(t, source, `{"profiles":[{"id":"first-id","name":"Unique","inputs":[]},{"id":"First","name":"Second","inputs":[]},{"id":"same","name":"same","inputs":[]}]}`)
		c, err := NewController(t.Context(), m, source)
		if err != nil {
			t.Fatal(err)
		}
		beforeBytes := readControllerStore(t, home)
		for _, selector := range []string{"first-id", "First", "Second", "missing"} {
			before := controllerStatus(t, c)
			response := c.Dispatch(t.Context(), controllerRequest(MethodSelect, selector))
			want := ERR_PROFILE_AMBIGUOUS
			if selector == "missing" {
				want = profilesource.ERR_PROFILE_NOT_FOUND
			}
			after := controllerStatus(t, c)
			if response.OK || response.Error.Code != want || after.Generation != before.Generation || !reflect.DeepEqual(after.ActiveProfile, before.ActiveProfile) {
				t.Fatalf("selector %q: %+v", selector, response)
			}
		}
		for _, selector := range []string{"same", "Unique", "second-id"} {
			response := c.Dispatch(t.Context(), controllerRequest(MethodSelect, selector))
			if !response.OK {
				t.Fatalf("unique %q: %+v", selector, response)
			}
			selected := response.Result.(SelectionResult).ActiveProfile
			if selector == "second-id" && (selected.SourceRef != first.Source.Hash || selected.ProfileIndex != 2) {
				t.Fatalf("inconsistent selection: %+v", selected)
			}
		}
		if !reflect.DeepEqual(beforeBytes, readControllerStore(t, home)) {
			t.Fatal("selection wrote store")
		}
		fresh, err := NewController(t.Context(), m, source)
		if err != nil {
			t.Fatal(err)
		}
		if s := controllerStatus(t, fresh); s.ActiveProfile.SourceRef != second.Source.Hash || s.ActiveProfile.ProfileIndex != 3 {
			t.Fatalf("runtime choice persisted: %+v", s)
		}
		if err := os.WriteFile(filepath.Join(home, "azerlay/sources", second.Source.Hash+".azeron"), []byte("corrupt"), 0600); err != nil {
			t.Fatal(err)
		}
		before := controllerStatus(t, c)
		response := c.Dispatch(t.Context(), controllerRequest(MethodSelect, "second-id"))
		after := controllerStatus(t, c)
		if response.OK || response.Error.Code != profilesource.ERR_PROFILE_STORAGE || after.Generation != before.Generation || !reflect.DeepEqual(before.ActiveProfile, after.ActiveProfile) {
			t.Fatalf("corrupt candidate source: %+v", response)
		}
		fresh, err = NewController(t.Context(), m, source)
		if err != nil {
			t.Fatal(err)
		}
		if s := controllerStatus(t, fresh); s.ActiveProfile != nil || s.DegradedReasons[3].Code != profilesource.ERR_PROFILE_STORAGE {
			t.Fatalf("corrupt startup: %+v", s)
		}
	})
	t.Run("concurrent publication", func(t *testing.T) {
		m, _ := controllerManager(t, "")
		source := profilesource.NewImportedSource(t.TempDir())
		first := importControllerProfiles(t, source, controllerBundle)
		second := importControllerProfiles(t, source, `{"id":"third-id","name":"Third","inputs":[]}`)
		c, err := NewController(t.Context(), m, source)
		if err != nil {
			t.Fatal(err)
		}
		var workers sync.WaitGroup
		for range 20 {
			workers.Go(func() {
				for _, selector := range []string{"first-id", "third-id"} {
					if response := c.Dispatch(t.Context(), controllerRequest(MethodSelect, selector)); !response.OK {
						t.Errorf("concurrent select: %+v", response)
					}
					s := controllerStatus(t, c)
					p := s.ActiveProfile
					if p.ProfileIndex != 1 || !((p.SourceRef == first.Source.Hash && *p.Name == "First") || (p.SourceRef == second.Source.Hash && *p.Name == "Third")) {
						t.Errorf("torn publication: %+v", p)
					}
					*p.Name = "caller modification"
					c.Dispatch(t.Context(), controllerRequest(MethodToggle, ""))
				}
			})
		}
		workers.Wait()
	})
}

func readControllerStore(t *testing.T, home string) map[string]string {
	t.Helper()
	contents := make(map[string]string)
	err := filepath.WalkDir(home, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		contents[path] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return contents
}
