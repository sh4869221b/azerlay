package profilesource

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stateFixture(t *testing.T) (LocalStateSelection, LocalRef, LocalCandidate, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", home)
	_, source, _ := localFixture(t)
	candidate, err := admitLocal([]byte(savedLocalJSON), source.Local.File)
	if err != nil {
		t.Fatal(err)
	}
	selection := LocalStateSelection{"auto", source.Local.Root, source.Local.Device, source.Local.File}
	return selection, source.Local, candidate, filepath.Join(home, "azerlay", "last-good.json")
}

func TestLocalStateRestoreMissingSource(t *testing.T) {
	selection, ref, candidate, path := stateFixture(t)
	if _, _, err := RestoreLocalState(selection); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing state = %v", err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("read-only restore created state directory")
	}
	if err := SaveLocalState(context.Background(), selection, ref, candidate); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		path string
		mode os.FileMode
	}{{path, 0600}, {filepath.Dir(path), os.ModeDir | 0700}} {
		info, err := os.Stat(check.path)
		if err != nil || info.Mode() != check.mode {
			t.Fatalf("private mode = %v %v", info, err)
		}
	}
	if err := os.Remove(filepath.Join(ref.Root, localRelativePath(ref))); err != nil {
		t.Fatal(err)
	}
	restored, got, err := RestoreLocalState(selection)
	if err != nil || restored != ref || !bytes.Equal(got.Original(), candidate.Original()) || got.Bundle().Source.SourceScope != "azeron-software-local-json" {
		t.Fatalf("restore = %v %v", restored, err)
	}
	for _, changed := range []LocalStateSelection{{"local", ref.Root, ref.Device, ref.File}, {"auto", t.TempDir(), ref.Device, ref.File}, {"auto", ref.Root, "other", ref.File}, {"auto", ref.Root, ref.Device, "profile_b.json"}} {
		if _, _, err := RestoreLocalState(changed); err == nil {
			t.Fatalf("accepted changed selection: %+v", changed)
		}
	}
}

func TestLocalStateAutoAmbiguityRetainsMatchingRoot(t *testing.T) {
	selection, ref, candidate, _ := stateFixture(t)
	home, config := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	ref.Root = filepath.Join(config, "Azeron Software")
	selection.StorePath = ""
	for _, root := range []string{ref.Root, filepath.Join(home, ".config", "Azeron Software")} {
		path := filepath.Join(root, localRelativePath(ref))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, candidate.Original(), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := SaveLocalState(context.Background(), selection, ref, candidate); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveLocalRef("", ref.Device, ref.File); err == nil {
		t.Fatal("expected ambiguous roots")
	}
	got, _, err := RestoreLocalState(selection)
	if err != nil || got != ref {
		t.Fatalf("matching ambiguous restore = %v %v", got, err)
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, _, err := RestoreLocalState(selection); err == nil {
		t.Fatal("accepted ineligible stored root")
	}
}

func TestLocalStateRejectsInvalidEnvelope(t *testing.T) {
	selection, ref, candidate, path := stateFixture(t)
	if err := SaveLocalState(context.Background(), selection, ref, candidate); err != nil {
		t.Fatal(err)
	}
	good, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []string{"schema", "release", "scope", "raw", "filename", "decoded_limit", "envelope_limit", "json"} {
		t.Run(test, func(t *testing.T) {
			var envelope localStateEnvelope
			if err := json.Unmarshal(good, &envelope); err != nil {
				t.Fatal(err)
			}
			switch test {
			case "schema":
				envelope.SchemaVersion = 2
			case "release":
				envelope.Release = "2.0.3"
			case "scope":
				envelope.Scope = "export"
			case "raw":
				envelope.Original = []byte(`{"id":"a"}`)
			case "filename":
				envelope.Ref.File = "profile_b.json"
			case "decoded_limit":
				envelope.Original = bytes.Repeat([]byte(" "), localSourceLimit+1)
			}
			data, err := json.Marshal(envelope)
			if err != nil {
				t.Fatal(err)
			}
			if test == "envelope_limit" {
				data = []byte(strings.Repeat(" ", localStateLimit+1))
			}
			if test == "json" {
				data = []byte("{")
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			_, got, err := RestoreLocalState(selection)
			if err == nil || err.Error() != ERR_PROFILE_LOCAL_STATE || len(got.Original()) != 0 {
				t.Fatalf("invalid restore = %v", err)
			}
		})
	}
}

func TestLocalStateFailurePreservesPreviousBytes(t *testing.T) {
	selection, ref, candidate, path := stateFixture(t)
	if err := SaveLocalState(context.Background(), selection, ref, candidate); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	otherSelection, otherRef := selection, ref
	otherSelection.File, otherRef.File = "profile_b.json", "profile_b.json"
	if err := SaveLocalState(context.Background(), otherSelection, otherRef, candidate); err == nil {
		t.Fatal("persisted candidate under a different filename")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := SaveLocalState(ctx, selection, ref, candidate); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel = %v", err)
	}
	publication := &publicationCancelContext{Context: context.Background(), directory: filepath.Dir(path)}
	if err := SaveLocalState(publication, selection, ref, candidate); !errors.Is(err, context.Canceled) || !publication.staged {
		t.Fatalf("publication cancellation = %v, staged = %v", err, publication.staged)
	}
	if err := os.Chmod(filepath.Dir(path), 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(filepath.Dir(path), 0700)
	if err := SaveLocalState(context.Background(), selection, ref, candidate); err == nil {
		t.Fatal("accepted unwritable state")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("previous state changed: %v", err)
	}
}

type publicationCancelContext struct {
	context.Context
	directory string
	staged    bool
}

func (ctx *publicationCancelContext) Err() error {
	entries, err := os.ReadDir(ctx.directory)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".last-good.") {
			ctx.staged = true
			return context.Canceled
		}
	}
	return nil
}

func TestLocalStateRejectsUnsafeAndOverlappingDestinations(t *testing.T) {
	for _, test := range []string{"file_mode", "directory_mode", "file_symlink", "ancestor_alias", "direct_overlap"} {
		t.Run(test, func(t *testing.T) {
			selection, ref, candidate, path := stateFixture(t)
			if err := SaveLocalState(context.Background(), selection, ref, candidate); err != nil {
				t.Fatal(err)
			}
			switch test {
			case "file_mode":
				if err := os.Chmod(path, 0644); err != nil {
					t.Fatal(err)
				}
			case "directory_mode":
				if err := os.Chmod(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
			case "file_symlink":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(ref.Root, localRelativePath(ref)), path); err != nil {
					t.Fatal(err)
				}
			case "ancestor_alias":
				alias := filepath.Join(t.TempDir(), "alias")
				if err := os.Symlink(ref.Root, alias); err != nil {
					t.Fatal(err)
				}
				t.Setenv("XDG_STATE_HOME", filepath.Join(alias, "new", "state"))
			case "direct_overlap":
				t.Setenv("XDG_STATE_HOME", filepath.Join(ref.Root, "new", "state"))
			}
			if err := SaveLocalState(context.Background(), selection, ref, candidate); err == nil || err.Error() != ERR_PROFILE_LOCAL_STATE {
				t.Fatalf("unsafe save = %v", err)
			}
			if _, _, err := RestoreLocalState(selection); err == nil {
				t.Fatal("unsafe restore succeeded")
			}
			if _, err := os.Stat(filepath.Join(ref.Root, "new")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("created state under userData")
			}
		})
	}
}
