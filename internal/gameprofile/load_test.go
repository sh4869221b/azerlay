package gameprofile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/sh4869221b/azerlay/internal/profile"
)

func writeProfile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadReplacement(t *testing.T) {
	t.Parallel()
	builtIn := fstest.MapFS{"assets/example.toml": {Data: []byte(`schema_version = 1
id = "sample"
name = "Embedded"
locale = "en"
[bindings]
KEY_U = "Embedded action"
[controls]
"input:15:single" = "Embedded control"
`)}}
	dir := t.TempDir()
	writeProfile(t, dir, "renamed.toml", `schema_version = 1
id = "sample"
name = "User"
locale = "ja"
[bindings]
KEY_Q = "Inert mapping"
`)
	catalog, err := loadFromFS(builtIn, dir)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := catalog.Lookup("sample")
	if !ok || d.Name != "User" || d.Locale != "ja" || len(d.Bindings) != 1 || len(d.Controls) != 0 || d.Bindings[profile.CanonicalCode("KEY_Q")] != "Inert mapping" {
		t.Fatalf("whole replacement failed: %+v, found=%t", d, ok)
	}
	if _, ok := d.Bindings[profile.CanonicalCode("KEY_U")]; ok {
		t.Fatal("embedded-only binding survived replacement")
	}
	d.Bindings[profile.CanonicalCode("KEY_Q")] = "changed"
	again, _ := catalog.Lookup("sample")
	if again.Bindings[profile.CanonicalCode("KEY_Q")] != "Inert mapping" {
		t.Fatal("Lookup returned shared mapping")
	}
	if _, ok := catalog.Lookup("../sample"); ok {
		t.Fatal("unknown ID unexpectedly resolved")
	}
}

func TestLoadMissingAndDuplicate(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "missing")
	catalog, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := catalog.Lookup("generic"); !ok {
		t.Fatal("embedded generic missing")
	}
	if d, ok := catalog.Lookup("unknown"); ok || len(d.Bindings) != 0 || len(d.Controls) != 0 {
		t.Fatalf("unknown ID: %+v, found=%t", d, ok)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("load wrote missing directory: %v", err)
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	content := "schema_version=1\nid=\"same\"\nname=\"A\"\n"
	writeProfile(t, dir, "a.toml", content)
	writeProfile(t, dir, "b.toml", content)
	if _, err := Load(dir); !hasStage(err, "duplicate") {
		t.Fatalf("duplicate user ID: %v", err)
	}
	builtIn := fstest.MapFS{
		"assets/a.toml": {Data: []byte(content)},
		"assets/b.toml": {Data: []byte(content)},
	}
	if _, err := loadFromFS(builtIn, t.TempDir()); !hasStage(err, "duplicate") {
		t.Fatalf("duplicate embedded ID: %v", err)
	}
}

func TestParseInvalid(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, content, stage string }{
		{"malformed", "[", "decode"},
		{"missing schema", `id="x"
name="X"`, "schema"},
		{"new schema", `schema_version=2
id="x"
name="X"`, "schema"},
		{"wrong type", `schema_version=1
id=7
name="X"`, "decode"},
		{"unknown field", `schema_version=1
id="x"
name="X"
extra=1`, "decode"},
		{"empty id", `schema_version=1
id="  "
name="X"`, "validation"},
		{"invalid binding", "schema_version=1\nid=\"x\"\nname=\"X\"\n[bindings]\nkey_U=\"x\"", "validation"},
		{"invalid control", "schema_version=1\nid=\"x\"\nname=\"X\"\n[controls]\n\"input:015:single\"=\"x\"", "validation"},
		{"unknown trigger", "schema_version=1\nid=\"x\"\nname=\"X\"\n[controls]\n\"input:15:hold\"=\"x\"", "validation"},
		{"extra segment", "schema_version=1\nid=\"x\"\nname=\"X\"\n[controls]\n\"input:15:single:x\"=\"x\"", "validation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeProfile(t, dir, "invalid.toml", tc.content)
			if _, err := Load(dir); !hasStage(err, tc.stage) {
				t.Fatalf("Load: %v, want stage %s", err, tc.stage)
			}
		})
	}
}

func hasStage(err error, stage string) bool {
	var gameErr *Error
	return errors.As(err, &gameErr) && gameErr.Code == ERR_GAME_PROFILE_INVALID && gameErr.Stage == stage
}
