package control

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/profilesource"
)

func TestConfiguredProfileSelection(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	source := profilesource.NewImportedSource(home)
	saved := importControllerProfiles(t, source, controllerBundle)
	before := readControllerStore(t, home)
	for _, tc := range []struct {
		name     string
		settings config.Profile
		index    int
		id       string
	}{
		{"auto saved ordinal", config.Profile{Source: "auto"}, 2, "second-id"},
		{"imported saved ordinal", config.Profile{Source: "imported"}, 2, "second-id"},
		{"auto configured ID", config.Profile{Source: "auto", SelectedID: "first-id"}, 1, "first-id"},
		{"imported configured ID", config.Profile{Source: "imported", SelectedID: "first-id"}, 1, "first-id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selected, p, err := ResolveConfiguredProfile(t.Context(), source, tc.settings)
			if err != nil || selected.Source != saved.Source || selected.ProfileIndex != tc.index || p.ID == nil || *p.ID != tc.id {
				t.Fatalf("selection=%+v profile=%+v err=%v", selected, p, err)
			}
		})
	}
	if !reflect.DeepEqual(before, readControllerStore(t, home)) {
		t.Fatal("configured profile resolution changed storage")
	}
}

func TestConfiguredProfileFailure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		input    string
		settings config.Profile
		corrupt  bool
		code     string
	}{
		{"no saved selection", "", config.Profile{Source: "auto"}, false, profilesource.ERR_PROFILE_NOT_FOUND},
		{"name is not ID", controllerBundle, config.Profile{Source: "imported", SelectedID: "First"}, false, profilesource.ERR_PROFILE_NOT_FOUND},
		{"duplicate ID", `{"profiles":[{"id":"duplicate","inputs":[]},{"id":"duplicate","inputs":[]}]}`, config.Profile{Source: "auto", SelectedID: "duplicate"}, false, ERR_PROFILE_AMBIGUOUS},
		{"local does not fall back", controllerBundle, config.Profile{Source: "local"}, false, ERR_PROFILE_SOURCE_UNAVAILABLE},
		{"corrupt selected source", controllerBundle, config.Profile{Source: "imported"}, true, profilesource.ERR_PROFILE_STORAGE},
		{"corrupt ID search source", controllerBundle, config.Profile{Source: "imported", SelectedID: "first-id"}, true, profilesource.ERR_PROFILE_STORAGE},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			source := profilesource.NewImportedSource(home)
			if tc.input != "" {
				selected := importControllerProfiles(t, source, tc.input)
				if tc.corrupt {
					if err := os.WriteFile(filepath.Join(home, "azerlay/sources", selected.Source.Hash+".azeron"), []byte("corrupt"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			before := readControllerStore(t, home)
			selected, p, err := ResolveConfiguredProfile(t.Context(), source, tc.settings)
			if err == nil || err.Error() != tc.code || selected != (profilesource.Selection{}) || p.ID != nil || len(p.Controls) != 0 {
				t.Fatalf("selection=%+v profile=%+v err=%v", selected, p, err)
			}
			if !reflect.DeepEqual(before, readControllerStore(t, home)) {
				t.Fatal("failed resolution changed storage")
			}
			if tc.input == "" {
				if _, err := os.Stat(filepath.Join(home, "azerlay")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("read created application directory: %v", err)
				}
			}
		})
	}
}

func TestConfiguredProfilePermissionCause(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("permission-denied fixture requires an unprivileged process")
	}
	t.Parallel()
	for _, selectedID := range []string{"", "first-id"} {
		t.Run("selected_id="+selectedID, func(t *testing.T) {
			home := t.TempDir()
			source := profilesource.NewImportedSource(home)
			importControllerProfiles(t, source, controllerBundle)
			m, _ := controllerManager(t, "selected_id = \""+selectedID+"\"\n")
			if err := os.Chmod(home, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(home, 0700); err != nil {
					t.Error(err)
				}
			})
			_, _, err := ResolveConfiguredProfile(t.Context(), source, m.Snapshot().Config.Profile)
			var failure *profilesource.Error
			if !errors.Is(err, os.ErrPermission) || !errors.As(err, &failure) || failure.Code != profilesource.ERR_PROFILE_STORAGE {
				t.Fatalf("permission cause not preserved: %v", err)
			}
			c, err := NewController(t.Context(), m, source)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(c.selectionFailure, NewError(profilesource.ERR_PROFILE_STORAGE)) || errors.Is(c.selectionFailure, os.ErrPermission) {
				t.Fatalf("controller did not project safe storage error: %+v", c.selectionFailure)
			}
		})
	}
}
