package profilesource

import (
	"bytes"
	"context"
	"encoding/base64"
	"github.com/sh4869221b/azerlay/internal/profile"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const savedLocalJSON = `{"id":"a","name":"Synthetic","version":1,"inputs":[],"isSoftware":true,"metaData":{"changedLogs":[{"softwareVersion":"2.0.2"}]}}`

func localFixture(t *testing.T) (*AzeronLocalSource, SourceRef, string) {
	t.Helper()
	root := t.TempDir()
	ref := SourceRef{Local: LocalRef{Root: root, Device: "device", File: "profile_a.json"}}
	path := filepath.Join(root, localRelativePath(ref.Local))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(savedLocalJSON), 0644); err != nil {
		t.Fatal(err)
	}
	return NewAzeronLocalSource(root), ref, path
}
func TestLocalReadOnlyAndExportBoundary(t *testing.T) {
	t.Parallel()
	s, ref, path := localFixture(t)
	if err := os.Chmod(path, 0444); err != nil {
		t.Fatal(err)
	}
	for dir := filepath.Dir(path); dir != filepath.Dir(s.root); dir = filepath.Dir(dir) {
		if err := os.Chmod(dir, 0555); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(dir, 0755)
	}
	c, err := s.LoadCandidate(context.Background(), ref)
	if err != nil || !bytes.Equal(c.Original(), []byte(savedLocalJSON)) || c.Bundle().Source.SourceScope != "azeron-software-local-json" {
		t.Fatalf("candidate = %#v %v", c, err)
	}
	list, err := s.Discover(context.Background())
	if err != nil || len(list) != 1 || list[0].Ref != ref || list[0].ProfileCount != 1 {
		t.Fatalf("catalog: %#v %v", list, err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatalf("source changed: %v %v", entries, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, []byte(savedLocalJSON)) {
		t.Fatal("source bytes changed")
	}
	for _, scope := range []string{"azeron-software-local-json"} {
		p, err := PrepareText(savedLocalJSON, profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: scope})
		if err == nil || len(p.original) != 0 {
			t.Fatal("local became Prepared")
		}
	}
	if _, err := NewImportedSource(t.TempDir()).Import(context.Background(), Prepared{}, 1, Origin{}); err == nil {
		t.Fatal("zero Prepared imported")
	}
}
func TestLocalReadRejectsUnsupported(t *testing.T) {
	t.Parallel()
	for _, data := range []string{base64.StdEncoding.EncodeToString([]byte(savedLocalJSON)), `"` + strings.ReplaceAll(savedLocalJSON, `"`, `\"`) + `"`, savedLocalJSON + `{}`, strings.Replace(savedLocalJSON, `"inputs":[]`, `"inputs":[`+strings.Repeat(`{},`, 256)+`{}]`, 1), strings.Replace(savedLocalJSON, `"name":"Synthetic"`, `"name":"`+strings.Repeat("a", 65537)+`"`, 1), string([]byte{0xff}), `{`, strings.Replace(savedLocalJSON, `"version":1`, `"version":2`, 1), strings.Replace(savedLocalJSON, `"id":"a"`, `"id":"b"`, 1), strings.Replace(savedLocalJSON, `"inputs":[]`, `"inputs":[1]`, 1), strings.Replace(savedLocalJSON, `"id":"a"`, `"id":"a","id":"a"`, 1), `{"profiles":[` + savedLocalJSON + `]}`, strings.Repeat(" ", localSourceLimit+1)} {
		s, ref, path := localFixture(t)
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
		c, err := s.LoadCandidate(context.Background(), ref)
		if err == nil || len(c.original) != 0 {
			t.Fatal("unsupported candidate published")
		}
		list, err := s.Discover(context.Background())
		if err != nil || len(list) != 0 {
			t.Fatal("unsupported catalog entry admitted")
		}
	}
}
func TestLocalReadRejectsRaces(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"opened", "read", "between"} {
		for _, replacement := range []bool{false, true} {
			t.Run(phase+map[bool]string{true: " replacement", false: " inplace"}[replacement], func(t *testing.T) {
				s, ref, path := localFixture(t)
				changed := false
				s.hook = func(event string) {
					if event != phase || changed {
						return
					}
					changed = true
					data := strings.Replace(savedLocalJSON, "Synthetic", "Different", 1)
					if replacement {
						temp := path + ".tmp"
						if err := os.WriteFile(temp, []byte(data), 0644); err != nil {
							t.Fatal(err)
						}
						if err := os.Rename(temp, path); err != nil {
							t.Fatal(err)
						}
					} else {
						if err := os.WriteFile(path, []byte(data), 0644); err != nil {
							t.Fatal(err)
						}
					}
				}
				c, err := s.LoadCandidate(context.Background(), ref)
				if err == nil || len(c.original) != 0 {
					t.Fatal("changed candidate published")
				}
				requireReadError(t, err, ERR_PROFILE_LOCAL_READ)
			})
		}
	}
}
func TestLocalReadFileBoundaryAndCancellation(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"fifo", "directory", "escape"} {
		s, ref, path := localFixture(t)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		switch kind {
		case "fifo":
			if err := syscall.Mkfifo(path, 0600); err != nil {
				t.Fatal(err)
			}
		case "directory":
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
		case "escape":
			outside := filepath.Join(t.TempDir(), "external")
			if err := os.WriteFile(outside, []byte(savedLocalJSON), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, path); err != nil {
				t.Fatal(err)
			}
		}
		start := time.Now()
		c, err := s.LoadCandidate(context.Background(), ref)
		if err == nil || len(c.original) != 0 || time.Since(start) > time.Second {
			t.Fatal("invalid file blocked or admitted")
		}
	}
	s, ref, _ := localFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	s.hook = func(phase string) {
		if phase == "between" {
			cancel()
		}
	}
	c, err := s.LoadCandidate(ctx, ref)
	if err != context.Canceled || len(c.original) != 0 {
		t.Fatalf("cancellation: %v", err)
	}
	for _, wrong := range []SourceRef{{Hash: strings.Repeat("a", 64)}, {Local: LocalRef{Root: t.TempDir(), Device: "device", File: "profile_a.json"}}} {
		if _, err := s.Load(context.Background(), wrong); err == nil {
			t.Fatal("wrong ref accepted")
		}
	}
}
