package profilesource

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalRefBoundary(t *testing.T) {
	t.Parallel()
	valid := SourceRef{Local: LocalRef{Root: t.TempDir(), Device: "device", File: "profile_a.json"}}
	if !validLocalRef(valid) {
		t.Fatal("complete local ref rejected")
	}
	for _, ref := range []SourceRef{
		{}, {Hash: strings.Repeat("a", 64)}, {Hash: strings.Repeat("a", 64), Local: valid.Local},
		{Local: LocalRef{Root: "relative", Device: "device", File: "profile_a.json"}},
		{Local: LocalRef{Root: valid.Local.Root, Device: "device"}},
		{Local: LocalRef{Root: valid.Local.Root, Device: "..", File: "profile_a.json"}},
		{Local: LocalRef{Root: valid.Local.Root, Device: "device", File: "a/b"}},
		{Local: LocalRef{Root: valid.Local.Root, Device: "device", File: "a\\b"}},
		{Local: LocalRef{Root: valid.Local.Root, Device: "device", File: "a\x00b"}},
	} {
		if validLocalRef(ref) {
			t.Fatalf("invalid ref accepted: %+v", ref)
		}
	}
	home, selected, _ := loadFixture(t)
	for _, operation := range []string{"load", "watch"} {
		for _, ref := range []SourceRef{valid, {Hash: selected.Source.Hash, Local: valid.Local}} {
			err := readOperation(t, NewImportedSource(home), readRequest{ctx: context.Background(), operation: operation, ref: ref})
			requireReadError(t, err, ERR_PROFILE_STORAGE)
		}
	}
}

func TestLocalExplicitRoot(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, path := range []string{"missing/userData", "~/$HOME/userData"} {
		want, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		ref, err := ResolveLocalRef(path, "device", "profile_a.json")
		if err != nil || ref != (LocalRef{Root: want, Device: "device", File: "profile_a.json"}) {
			t.Fatalf("explicit ref = %+v, %v", ref, err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("resolver created path: %v", err)
		}
	}
	if ref, err := ResolveLocalRef("missing", "", ""); err == nil || ref != (LocalRef{}) {
		t.Fatal("missing pair selected")
	}
}

func TestLocalAutomaticRoots(t *testing.T) {
	home, xdg := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	homeRoot := filepath.Join(home, ".config", "Azeron Software")
	xdgRoot := filepath.Join(xdg, "Azeron Software")
	if ref, err := ResolveLocalRef("", "device", "profile_a.json"); err == nil || ref != (LocalRef{}) {
		t.Fatal("missing store selected")
	}
	for _, root := range []string{homeRoot, xdgRoot} {
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Fatal("detection created store")
		}
	}
	makeProfile := func(root string) {
		path := filepath.Join(root, localRelativePath(LocalRef{Device: "device", File: "profile_a.json"}))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	makeProfile(homeRoot)
	ref, err := ResolveLocalRef("", "device", "profile_a.json")
	if err != nil || ref.Root != homeRoot {
		t.Fatalf("HOME ref = %+v, %v", ref, err)
	}
	makeProfile(xdgRoot)
	if ref, err := ResolveLocalRef("", "device", "profile_a.json"); err == nil || ref != (LocalRef{}) {
		t.Fatal("ambiguous roots selected")
	}
	alias := filepath.Join(t.TempDir(), "config")
	if err := os.Symlink(filepath.Join(home, ".config"), alias); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", alias)
	ref, err = ResolveLocalRef("", "device", "profile_a.json")
	if err != nil || ref.Root != filepath.Join(alias, "Azeron Software") {
		t.Fatalf("alias ref = %+v, %v", ref, err)
	}
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	roots, err := localRoots("")
	if err != nil || len(roots) != 1 || roots[0] != homeRoot {
		t.Fatalf("duplicate roots = %v, %v", roots, err)
	}
	t.Setenv("XDG_CONFIG_HOME", "relative")
	roots, err = localRoots("")
	if err != nil || len(roots) != 1 || roots[0] != homeRoot {
		t.Fatalf("relative XDG = %v, %v", roots, err)
	}
}

func TestLocalDetectionKeepsUnadmittedRootAmbiguous(t *testing.T) {
	home, xdg := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	for index, root := range []string{filepath.Join(home, ".config", "Azeron Software"), filepath.Join(xdg, "Azeron Software")} {
		path := filepath.Join(root, localRelativePath(LocalRef{Device: "device", File: "profile_a.json"}))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			if err := os.WriteFile(path, []byte(savedLocalJSON), 0600); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
		}
	}
	if ref, err := ResolveLocalRef("", "device", "profile_a.json"); err == nil || ref != (LocalRef{}) {
		t.Fatal("unadmitted pathname hid root ambiguity")
	}
}
