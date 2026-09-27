package gameprofile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profilesource"
)

func TestIntegrationReload(t *testing.T) {
	t.Parallel()
	export, err := os.ReadFile("../profileadapter/testdata/unbound.input.json")
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := profilesource.PrepareText(string(export), profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export"})
	if err != nil {
		t.Fatal(err)
	}
	control := prepared.Bundle().Profiles[0].Controls[0]
	if control.SourceIdentity.Invalid || control.SourceIdentity.InputID == nil || *control.SourceIdentity.InputID != 4 || len(control.Bindings) == 0 || control.Bindings[0].Kind != profile.BindingUnbound {
		t.Fatalf("unexpected normalized fixture: %+v", control)
	}
	binding := control.Bindings[0]
	dir := t.TempDir()
	path := filepath.Join(dir, "game.toml")
	writeProfile(t, dir, "game.toml", `schema_version=1
id="integration"
name="Synthetic A"
[controls]
"input:4:single"="Action A"
`)
	var store Store
	if err := store.Reload(dir); err != nil {
		t.Fatal(err)
	}
	resolved := func(id string) Resolved {
		game, _ := store.Snapshot().Lookup(id)
		return Resolve(control, binding, game)
	}
	if got := resolved("integration"); got != (Resolved{Label: "Action A", BindingDisplay: "Unbound"}) {
		t.Fatalf("initial resolution: %+v", got)
	}
	if got := resolved("missing"); got != (Resolved{Label: "synthetic", BindingDisplay: "Unbound"}) {
		t.Fatalf("missing-game fallback: %+v", got)
	}
	writeProfile(t, dir, "replacement.toml", "schema_version = [")
	if err := os.Rename(filepath.Join(dir, "replacement.toml"), path); err != nil {
		t.Fatal(err)
	}
	if err := store.Reload(dir); !hasStage(err, "decode") {
		t.Fatalf("invalid reload: %v", err)
	}
	if got := resolved("integration"); got != (Resolved{Label: "Action A", BindingDisplay: "Unbound"}) {
		t.Fatalf("last-good resolution: %+v", got)
	}
	writeProfile(t, dir, "replacement.toml", `schema_version=1
id="integration"
name="Synthetic B"
[controls]
"input:4:single"="Action B"
`)
	if err := os.Rename(filepath.Join(dir, "replacement.toml"), path); err != nil {
		t.Fatal(err)
	}
	if err := store.Reload(dir); err != nil {
		t.Fatal(err)
	}
	if got := resolved("integration"); got != (Resolved{Label: "Action B", BindingDisplay: "Unbound"}) {
		t.Fatalf("repaired resolution: %+v", got)
	}
}
