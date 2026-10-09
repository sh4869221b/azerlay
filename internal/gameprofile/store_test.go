package gameprofile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
)

func TestStore(t *testing.T) {
	t.Parallel()
	var store Store
	if _, ok := store.Snapshot().Lookup("generic"); ok {
		t.Fatal("zero store is not empty")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "game.toml")
	writeProfile(t, dir, "game.toml", `schema_version=1
id="sample"
name="A"
locale="en"
[controls]
"input:15:single"="A control"
`)
	if err := store.Reload(dir); err != nil {
		t.Fatal(err)
	}
	snapshotA := store.Snapshot()
	a, ok := snapshotA.Lookup("sample")
	if !ok || a.Name != "A" || a.Locale != "en" || a.Controls[ControlKey{InputID: 15, Trigger: profile.TriggerSingle}] != "A control" {
		t.Fatalf("A: %+v, found=%t", a, ok)
	}
	a.Controls[ControlKey{InputID: 15, Trigger: profile.TriggerSingle}] = "caller edit"
	stillA, _ := store.Snapshot().Lookup("sample")
	if stillA.Controls[ControlKey{InputID: 15, Trigger: profile.TriggerSingle}] != "A control" {
		t.Fatal("Lookup allowed caller mutation of the store")
	}

	writeProfile(t, dir, "replacement.toml", `schema_version=1
id="sample"
name="Broken"
[controls]
"input:15:single"=7
`)
	if err := os.Rename(filepath.Join(dir, "replacement.toml"), path); err != nil {
		t.Fatal(err)
	}
	if err := store.Reload(dir); !hasStage(err, "decode") {
		t.Fatalf("invalid reload: %v", err)
	}
	if got, _ := store.Snapshot().Lookup("sample"); got.Name != "A" {
		t.Fatalf("invalid reload replaced A: %+v", got)
	}

	writeProfile(t, dir, "game.toml", `schema_version=1
id="sample"
name="B"
`)
	writeProfile(t, dir, "duplicate.toml", `schema_version=1
id="sample"
name="Duplicate"
`)
	if err := store.Reload(dir); !hasStage(err, "duplicate") {
		t.Fatalf("duplicate reload: %v", err)
	}
	if got, _ := store.Snapshot().Lookup("sample"); got.Name != "A" {
		t.Fatalf("duplicate reload replaced A: %+v", got)
	}
	if err := os.Remove(filepath.Join(dir, "duplicate.toml")); err != nil {
		t.Fatal(err)
	}
	if err := store.Reload(dir); err != nil {
		t.Fatal(err)
	}
	b, _ := store.Snapshot().Lookup("sample")
	if b.Name != "B" || b.Locale != "" || len(b.Controls) != 0 {
		t.Fatalf("repair did not publish B: %+v", b)
	}
	old, _ := snapshotA.Lookup("sample")
	if old.Name != "A" || old.Controls[ControlKey{InputID: 15, Trigger: profile.TriggerSingle}] != "A control" {
		t.Fatalf("old snapshot changed: %+v", old)
	}

	if err := store.Reload(filepath.Join(dir, "missing")); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Snapshot().Lookup("sample"); ok {
		t.Fatal("missing user directory retained previous game")
	}
	if _, ok := store.Snapshot().Lookup("generic"); !ok {
		t.Fatal("missing user directory omitted embedded generic")
	}

	writeProfile(t, dir, "invalid.toml", "[")
	var failedZero Store
	if err := failedZero.Reload(dir); !hasStage(err, "decode") {
		t.Fatalf("failed first reload: %v", err)
	}
	if _, ok := failedZero.Snapshot().Lookup("generic"); ok {
		t.Fatal("failed first reload changed zero store")
	}
}
