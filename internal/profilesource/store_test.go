package profilesource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/profile"
)

const storeInput = " \n" + `{"version":1e+09,"profiles":[{"id":"../duplicate","name":"First","inputs":[]},{"id":"../duplicate","name":"Second","inputs":[]}]}` + "\t"

// allow: SIZE_OK - Task 3 explicitly confines transaction regression coverage to
// store_test.go; process barriers, real fault fixtures and owned-tree assertions
// remain together rather than expanding the approved product-file scope.

func requireStorageError(t *testing.T, err error) {
	t.Helper()
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != ERR_PROFILE_STORAGE || err.Error() != ERR_PROFILE_STORAGE || errors.Unwrap(failure) == nil {
		t.Fatalf("storage error = %v", err)
	}
}

type storeEntry struct {
	Mode  os.FileMode
	Bytes string
	Inode uint64
}

func storeTree(t *testing.T, home string) map[string]storeEntry {
	t.Helper()
	result := make(map[string]storeEntry)
	err := filepath.WalkDir(home, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return errUnsafePath
		}
		value := storeEntry{Mode: info.Mode(), Inode: stat.Ino}
		switch {
		case info.Mode().IsRegular():
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value.Bytes = string(data)
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			value.Bytes = target
		}
		rel, err := filepath.Rel(home, path)
		if err != nil {
			return err
		}
		result[rel] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func storePrepared(t *testing.T, input string) Prepared {
	t.Helper()
	p, err := PrepareText(input, profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export"})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func storeBytes(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func storeIndex(t *testing.T, home string) diskIndex {
	t.Helper()
	i, err := decodeIndex(bytes.NewReader(storeBytes(t, filepath.Join(home, "azerlay/cache/source-index.json"))))
	if err != nil {
		t.Fatal(err)
	}
	return i
}
func TestImportedCommit(t *testing.T) {
	// Given: exact whitespace-bearing bytes, duplicate unsafe IDs and explicit index 2.
	home := t.TempDir()
	p := storePrepared(t, storeInput)
	s := NewImportedSource(home)
	s.now = func() time.Time { return time.Date(2026, 9, 8, 1, 2, 3, 123456789, time.UTC) }
	// When: import through the public transaction surface.
	got, err := s.Import(context.Background(), p, 2, Origin{Kind: "text"})
	// Then: committed identity, ordinal and every complete artifact match the input.
	if err != nil {
		t.Fatal(err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(storeInput)))
	if got != (Selection{Source: SourceRef{Hash: hash}, ProfileIndex: 2}) {
		t.Fatalf("selection = %+v", got)
	}
	index := storeIndex(t, home)
	if len(index.Sources) != 1 || index.Selected.ProfileIndex != 2 || index.Selected.SourceHash != hash {
		t.Fatalf("index = %+v", index)
	}
	wantSource := diskSource{SourceHash: hash, SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export", InputKind: "text", Origin: diskOrigin{Kind: "text"}, ImportedAt: "2026-09-08T01:02:03.123456789Z", DecoderVersion: "1", NormalizerVersion: "1", ModelSchemaVersion: 1, ProfileCount: 2, ExportVersion: tokenToDisk([]byte("1e+09"))}
	if !reflect.DeepEqual(index.Sources[0], wantSource) {
		t.Fatalf("metadata = %+v", index.Sources[0])
	}
	if !bytes.Equal(storeBytes(t, filepath.Join(home, "azerlay/sources", hash+".azeron")), []byte(storeInput)) {
		t.Fatal("original changed")
	}
	dir := filepath.Join(home, "azerlay/profiles", hash)
	readers := []io.Reader{bytes.NewReader(storeBytes(t, filepath.Join(dir, "p1.json"))), bytes.NewReader(storeBytes(t, filepath.Join(dir, "p2.json")))}
	bundle, err := decodeCacheSet(index.Sources[0], bytes.NewReader(storeBytes(t, filepath.Join(dir, "bundle.json"))), readers)
	if err != nil || !reflect.DeepEqual(bundle, p.Bundle()) {
		t.Fatalf("cache set differs: %v", err)
	}
	if err := filepath.WalkDir(filepath.Join(home, "azerlay"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		want := os.FileMode(0600)
		if entry.IsDir() {
			want = 0700
		}
		if info.Mode().Perm() != want {
			return fmt.Errorf("mode %s = %o", path, info.Mode().Perm())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
func TestImportedDuplicate(t *testing.T) {
	// Given: an already committed exact-byte source.
	home := t.TempDir()
	p := storePrepared(t, storeInput)
	s := NewImportedSource(home)
	s.now = func() time.Time { return time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC) }
	originalPath := "/private/synthetic/input.azeron"
	first, err := s.Import(context.Background(), p, 2, Origin{Kind: "file", Path: &originalPath})
	if err != nil {
		t.Fatal(err)
	}
	before := storeIndex(t, home)
	beforeTree := storeTree(t, home)
	s.now = func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) }
	path := filepath.Join(home, "azerlay/sources", first.Source.Hash+".azeron")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := PrepareReader(bytes.NewReader([]byte(storeInput)), p.Bundle().Source)
	if err != nil {
		t.Fatal(err)
	}
	// When: reimport identical bytes via a different route and choose index 1.
	got, err := s.Import(context.Background(), reader, 1, Origin{Kind: "stdin"})
	// Then: original inode and historical metadata survive; selection alone changes.
	if err != nil {
		t.Fatal(err)
	}
	after := storeIndex(t, home)
	if got.ProfileIndex != 1 || after.Selected.ProfileIndex != 1 || !reflect.DeepEqual(before.Sources, after.Sources) {
		t.Fatal("duplicate changed catalog or lost selection")
	}
	now, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(info, now) {
		t.Fatal("original rewritten")
	}
	afterTree := storeTree(t, home)
	for path, want := range beforeTree {
		if path != "azerlay/cache/source-index.json" && !reflect.DeepEqual(afterTree[path], want) {
			t.Fatalf("duplicate rewrote %s", path)
		}
	}
}
func TestImportedDuplicateCacheReuse(t *testing.T) {
	for _, kind := range []string{"missing bundle", "missing p2", "corrupt p1", "stale bundle", "historical compatible"} {
		t.Run(kind, func(t *testing.T) {
			// Given: first-import historical revisions plus a complete or damaged derived set.
			home := t.TempDir()
			p := storePrepared(t, storeInput)
			s := NewImportedSource(home)
			selected, err := s.Import(context.Background(), p, 2, Origin{Kind: "file"})
			if err != nil {
				t.Fatal(err)
			}
			index := storeIndex(t, home)
			index.Sources[0].DecoderVersion = "historic-decoder"
			index.Sources[0].NormalizerVersion = "historic-normalizer"
			index.Sources[0].ModelSchemaVersion = 7
			data, err := encodeIndex(index)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, "azerlay/cache/source-index.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(home, "azerlay/profiles", selected.Source.Hash)
			switch kind {
			case "missing bundle":
				if err := os.Remove(filepath.Join(dir, "bundle.json")); err != nil {
					t.Fatal(err)
				}
			case "missing p2":
				if err := os.Remove(filepath.Join(dir, "p2.json")); err != nil {
					t.Fatal(err)
				}
			case "corrupt p1":
				if err := os.WriteFile(filepath.Join(dir, "p1.json"), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			case "stale bundle":
				if err := os.WriteFile(filepath.Join(dir, "bundle.json"), []byte(`{"schema_version":999}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "historical compatible":
			default:
				t.Fatal("unknown cache fixture")
			}
			before := storeTree(t, home)
			// When: admitted duplicate import updates selection and replaces a missed whole set.
			if _, err := s.Import(context.Background(), p, 1, Origin{Kind: "text"}); err != nil {
				t.Fatal(err)
			}
			// Then: current complete caches, exact original reuse, historical metadata retention.
			after := storeIndex(t, home)
			if !reflect.DeepEqual(index.Sources, after.Sources) || after.Selected.ProfileIndex != 1 {
				t.Fatal("history or selection changed incorrectly")
			}
			readers := []io.Reader{bytes.NewReader(storeBytes(t, filepath.Join(dir, "p1.json"))), bytes.NewReader(storeBytes(t, filepath.Join(dir, "p2.json")))}
			bundle, err := decodeCacheSet(after.Sources[0], bytes.NewReader(storeBytes(t, filepath.Join(dir, "bundle.json"))), readers)
			if err != nil || !reflect.DeepEqual(bundle, p.Bundle()) {
				t.Fatalf("replacement incomplete: %v", err)
			}
			tree := storeTree(t, home)
			original := "azerlay/sources/" + selected.Source.Hash + ".azeron"
			if before[original] != tree[original] {
				t.Fatal("cache miss rewrote original")
			}
			if kind == "historical compatible" {
				for path, want := range before {
					if path != "azerlay/cache/source-index.json" && tree[path] != want {
						t.Fatalf("compatible artifact rewritten: %s", path)
					}
				}
			}
		})
	}
}

func TestImportedSelectionNoWrite(t *testing.T) {
	for _, ordinal := range []int{-1, 0, 3} {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			// Given: a lazy source and an invalid explicit ordinal.
			home := t.TempDir()
			s := NewImportedSource(home)
			p := storePrepared(t, storeInput)
			// When: attempt to import.
			_, err := s.Import(context.Background(), p, ordinal, Origin{Kind: "text"})
			// Then: failure and no directory, lock or file creation.
			requireStorageError(t, err)
			entries, readErr := os.ReadDir(home)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if len(entries) != 0 {
				t.Fatal("invalid selection mutated store")
			}
		})
	}
	for _, kind := range []string{"zero", "empty", "malformed", "canceled", "origin", "existing"} {
		t.Run(kind, func(t *testing.T) {
			// Given: a rejected candidate or a canceled import, optionally with old state.
			home := t.TempDir()
			s := NewImportedSource(home)
			p := storePrepared(t, storeInput)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			origin := Origin{Kind: "text"}
			ordinal := 1
			switch kind {
			case "zero":
				p = Prepared{}
			case "empty":
				p = storePrepared(t, `{"profiles":[]}`)
			case "malformed":
				var err error
				p, err = PrepareText("{", p.Bundle().Source)
				if err == nil {
					t.Fatal("malformed preparation succeeded")
				}
			case "canceled":
				cancel()
			case "origin":
				origin.Kind = "unsupported"
			case "existing":
				if _, err := s.Import(ctx, p, 2, origin); err != nil {
					t.Fatal(err)
				}
				ordinal = 0
			default:
				t.Fatal("unknown case")
			}
			before := storeTree(t, home)
			// When: attempt the invalid transaction.
			selection, err := s.Import(ctx, p, ordinal, origin)
			// Then: stable error with cause, zero selection, and byte/mode/inode-equal tree.
			requireStorageError(t, err)
			if selection != (Selection{}) {
				t.Fatal("failed import returned selection")
			}
			if kind == "canceled" && !errors.Is(err, context.Canceled) {
				t.Fatal("cancellation cause lost")
			}
			if !reflect.DeepEqual(before, storeTree(t, home)) {
				t.Fatal("invalid candidate wrote state")
			}
		})
	}
}

func TestImportedCommitFailure(t *testing.T) {
	for _, phase := range []string{"write", "sync", "close", "rename"} {
		for _, suffix := range []string{"bundle.json", "p1.json", "p2.json", ".azeron", "source-index.json"} {
			t.Run(phase+"/"+suffix, func(t *testing.T) {
				// Given: old committed state and a fault armed on an actual staged operation.
				home := t.TempDir()
				s := NewImportedSource(home)
				p := storePrepared(t, storeInput)
				if _, err := s.Import(context.Background(), p, 2, Origin{Kind: "text"}); err != nil {
					t.Fatal(err)
				}
				before := storeTree(t, home)
				incoming := storePrepared(t, storeInput+"\n")
				fired := false
				var hookErr error
				s.hook = func(event publicationEvent) {
					if event.phase != phase || !strings.HasSuffix(event.path, suffix) {
						return
					}
					fired = true
					switch phase {
					case "write":
						full, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
						if err != nil {
							hookErr = err
							return
						}
						hookErr = errors.Join(syscall.Dup2(int(full.Fd()), int(event.file.Fd())), full.Close())
					case "sync":
						read, write, err := os.Pipe()
						if err != nil {
							hookErr = err
							return
						}
						hookErr = errors.Join(syscall.Dup2(int(write.Fd()), int(event.file.Fd())), read.Close(), write.Close())
					case "close":
						hookErr = event.file.Close()
					case "rename":
						hookErr = os.Remove(filepath.Join(home, "azerlay", event.staging))
					default:
						hookErr = errors.New("unexpected phase")
					}
				}
				// When: the real Write/Sync/Close/Rename fails, not a mock early return.
				selection, err := s.Import(context.Background(), incoming, 1, Origin{Kind: "text"})
				// Then: the underlying syscall cause survives and all owned unpublished files disappear.
				if hookErr != nil {
					t.Fatal(hookErr)
				}
				if !fired {
					t.Fatal("fault did not execute")
				}
				requireStorageError(t, err)
				var want error
				switch phase {
				case "write":
					want = syscall.ENOSPC
				case "sync":
					want = syscall.EINVAL
				case "close":
					want = os.ErrClosed
				case "rename":
					want = os.ErrNotExist
				default:
					t.Fatal("unknown phase")
				}
				if !errors.Is(err, want) {
					t.Fatalf("cause = %v, want %v", errors.Unwrap(err), want)
				}
				if selection != (Selection{}) || !reflect.DeepEqual(before, storeTree(t, home)) {
					t.Fatal("failed commit changed old state or retained owned files")
				}
			})
		}
	}
	for _, kind := range []string{"corrupt index", "index directory", "original mismatch", "release conflict", "scope conflict", "count conflict", "index cap", "preserve orphan"} {
		t.Run(kind, func(t *testing.T) {
			// Given: old state with one concrete disk conflict/corruption or bounded overflow.
			home := t.TempDir()
			s := NewImportedSource(home)
			p := storePrepared(t, storeInput)
			selected, err := s.Import(context.Background(), p, 2, Origin{Kind: "text"})
			if err != nil {
				t.Fatal(err)
			}
			indexPath := filepath.Join(home, "azerlay/cache/source-index.json")
			index := storeIndex(t, home)
			switch kind {
			case "corrupt index":
				if err := os.WriteFile(indexPath, []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			case "index directory":
				if err := os.Rename(indexPath, indexPath+".saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(indexPath, 0700); err != nil {
					t.Fatal(err)
				}
			case "original mismatch":
				if err := os.WriteFile(filepath.Join(home, "azerlay/sources", selected.Source.Hash+".azeron"), []byte("broken"), 0600); err != nil {
					t.Fatal(err)
				}
			case "release conflict", "scope conflict", "count conflict":
				switch kind {
				case "release conflict":
					index.Sources[0].SoftwareRelease = "9.9.9"
				case "scope conflict":
					index.Sources[0].SourceScope = "other"
				case "count conflict":
					index.Sources[0].ProfileCount = 3
				default:
					t.Fatal("unknown conflict")
				}
				data, err := encodeIndex(index)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(indexPath, data, 0600); err != nil {
					t.Fatal(err)
				}
			case "index cap":
				// A valid index fits exactly; adding this new source must overflow before staging.
				data, err := encodeIndex(index)
				if err != nil {
					t.Fatal(err)
				}
				padding := strings.Repeat("x", maxIndexBytes-len(data)+2)
				index.Sources[0].Origin.Path = &padding
				data, err = encodeIndex(index)
				if err != nil {
					t.Fatal(err)
				}
				if len(data) != maxIndexBytes {
					t.Fatalf("cap fixture = %d", len(data))
				}
				if err := os.WriteFile(indexPath, data, 0600); err != nil {
					t.Fatal(err)
				}
				p = storePrepared(t, storeInput+"\n")
			case "preserve orphan":
				p = storePrepared(t, storeInput+"\n")
				hash := fmt.Sprintf("%x", sha256.Sum256(p.original))
				if err := os.WriteFile(filepath.Join(home, "azerlay/sources", hash+".azeron"), p.original, 0600); err != nil {
					t.Fatal(err)
				}
				s.hook = func(event publicationEvent) {
					if event.phase == "close" && event.path == "cache/source-index.json" {
						if err := event.file.Close(); err != nil {
							t.Error(err)
						}
					}
				}
			default:
				t.Fatal("unknown corruption")
			}
			before := storeTree(t, home)
			// When: import tries to update the committed selection.
			_, err = s.Import(context.Background(), p, 1, Origin{Kind: "text"})
			// Then: no reset, overwrite, adoption, or deletion of a pre-existing orphan.
			requireStorageError(t, err)
			if !reflect.DeepEqual(before, storeTree(t, home)) {
				t.Fatal("conflict mutated prior state")
			}
		})
	}
}

func TestImportedCancellationBeforeCommit(t *testing.T) {
	// Given: old state and cancellation delivered at the pre-commit barrier.
	home := t.TempDir()
	s := NewImportedSource(home)
	p := storePrepared(t, storeInput)
	if _, err := s.Import(context.Background(), p, 2, Origin{Kind: "text"}); err != nil {
		t.Fatal(err)
	}
	before := storeTree(t, home)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.hook = func(event publicationEvent) {
		if event.phase == "rename" && event.path == "cache/source-index.json" {
			cancel()
		}
	}
	// When: cancellation arrives after staging, but before the commit syscall.
	selection, err := s.Import(ctx, storePrepared(t, storeInput+"\n"), 1, Origin{Kind: "text"})
	// Then: no successful selection or adoption; cancellation remains inspectable.
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
	requireStorageError(t, err)
	if selection != (Selection{}) || !reflect.DeepEqual(before, storeTree(t, home)) {
		t.Fatal("canceled commit changed state")
	}
}

func TestImportedIndexLastPublication(t *testing.T) {
	// Given: old state and a subscribed pre-index-rename barrier.
	home := t.TempDir()
	s := NewImportedSource(home)
	old := storePrepared(t, storeInput)
	if _, err := s.Import(context.Background(), old, 2, Origin{Kind: "text"}); err != nil {
		t.Fatal(err)
	}
	before := storeBytes(t, filepath.Join(home, "azerlay/cache/source-index.json"))
	incoming := storePrepared(t, storeInput+"\n")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ready := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	defer func() { cancel(); <-done }() // Join even when a barrier assertion fails.
	s.hook = func(event publicationEvent) {
		if event.phase == "rename" && event.path == "cache/source-index.json" {
			close(ready)
			select {
			case <-release:
			case <-ctx.Done():
			}
		}
	}
	// When: import publishes artifacts but pauses before the index commit point.
	go func() { _, err := s.Import(ctx, incoming, 1, Origin{Kind: "text"}); done <- err; close(done) }()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// Then: an independent reader sees the exact old index although new bytes exist.
	if !bytes.Equal(before, storeBytes(t, filepath.Join(home, "azerlay/cache/source-index.json"))) {
		t.Error("index visible before commit")
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(incoming.original))
	if !bytes.Equal(incoming.original, storeBytes(t, filepath.Join(home, "azerlay/sources", hash+".azeron"))) {
		t.Error("original incomplete at barrier")
	}
	for _, name := range []string{"bundle.json", "p1.json", "p2.json"} {
		if len(storeBytes(t, filepath.Join(home, "azerlay/profiles", hash, name))) == 0 {
			t.Error("empty staged cache")
		}
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	index := storeIndex(t, home)
	if len(index.Sources) != 2 || index.Selected.SourceHash != hash || index.Selected.ProfileIndex != 1 {
		t.Fatalf("new index = %+v", index)
	}
}

func TestImportedConcurrentWriters(t *testing.T) {
	if mode := os.Getenv("AZERLAY_STORE_WORKER"); mode != "" {
		// Child process: exact pipe protocol is isolated to this test invocation.
		ready := os.NewFile(3, "ready")
		gate := os.NewFile(4, "gate")
		defer ready.Close()
		defer gate.Close()
		s := NewImportedSource(os.Getenv("AZERLAY_STORE_HOME"))
		p := storePrepared(t, storeInput+mode)
		s.hook = func(event publicationEvent) {
			if event.phase == "lock" && mode == "\n" {
				if err := syscall.Flock(int(event.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
					t.Fatalf("writer did not contend on stable lock: %v", err)
				}
				if _, err := ready.Write([]byte{'L'}); err != nil {
					t.Fatal(err)
				}
			}
			if event.phase == "rename" && event.path == "cache/source-index.json" {
				if _, err := ready.Write([]byte{'P'}); err != nil {
					t.Fatal(err)
				}
				var signal [1]byte
				if _, err := io.ReadFull(gate, signal[:]); err != nil {
					t.Fatal(err)
				}
				if signal[0] != 'G' {
					t.Fatal("bad gate")
				}
			}
		}
		if _, err := s.Import(context.Background(), p, 1, Origin{Kind: "text"}); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Run("serialized catalog", func(t *testing.T) {
		// Given: writer 1 paused under flock and writer 2 proving lock contention.
		home := t.TempDir()
		first := startStoreWorker(t, home, "")
		first.signal(t, 'P')
		second := startStoreWorker(t, home, "\n")
		second.signal(t, 'L')
		// When: release writer 1, then let writer 2 publish from its locked reread.
		first.release(t)
		first.wait(t)
		second.signal(t, 'P')
		old := storeIndex(t, home)
		if len(old.Sources) != 1 {
			t.Fatal("premature catalog update")
		}
		second.release(t)
		second.wait(t)
		// Then: exact first-commit order and last selection survive separate processes.
		index := storeIndex(t, home)
		hashes := []string{fmt.Sprintf("%x", sha256.Sum256([]byte(storeInput+" "))), fmt.Sprintf("%x", sha256.Sum256([]byte(storeInput+"\n")))}
		if len(index.Sources) != 2 || index.Sources[0].SourceHash != hashes[0] || index.Sources[1].SourceHash != hashes[1] || index.Selected.SourceHash != hashes[1] {
			t.Fatalf("lost/reordered source: %+v", index)
		}
	})
	t.Run("repeated interruptions", func(t *testing.T) {
		// Given: committed old state and two terminated writers at the commit barrier.
		home := t.TempDir()
		s := NewImportedSource(home)
		p := storePrepared(t, storeInput)
		if _, err := s.Import(context.Background(), p, 2, Origin{Kind: "text"}); err != nil {
			t.Fatal(err)
		}
		before := storeBytes(t, filepath.Join(home, "azerlay/cache/source-index.json"))
		for range 2 {
			worker := startStoreWorker(t, home, "\t")
			worker.signal(t, 'P')
			if err := worker.cmd.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			err := worker.cmd.Wait()
			worker.waited = true
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatalf("kill result = %v", err)
			}
		}
		// When: a fresh valid writer commits after both lock owners have exited.
		if !bytes.Equal(before, storeBytes(t, filepath.Join(home, "azerlay/cache/source-index.json"))) {
			t.Fatal("interruption adopted source")
		}
		if _, err := NewImportedSource(home).Import(context.Background(), storePrepared(t, storeInput+"\r"), 1, Origin{Kind: "text"}); err != nil {
			t.Fatal(err)
		}
		// Then: private orphan artifacts are ignored, never scanned into the catalog.
		index := storeIndex(t, home)
		if len(index.Sources) != 2 {
			t.Fatal("adopted interrupted source")
		}
		for _, source := range index.Sources {
			if source.SourceHash == fmt.Sprintf("%x", sha256.Sum256([]byte(storeInput+"\t"))) {
				t.Fatal("orphan adopted")
			}
		}
	})
}

type storeWorker struct {
	cmd         *exec.Cmd
	ready, gate *os.File
	output      bytes.Buffer
	waited      bool
}

func startStoreWorker(t *testing.T, home, mode string) *storeWorker {
	t.Helper()
	if mode == "" {
		mode = " "
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	gateRead, gateWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w := &storeWorker{cmd: exec.CommandContext(ctx, exe, "-test.run=^TestImportedConcurrentWriters$"), ready: readyRead, gate: gateWrite}
	w.cmd.Env = append(os.Environ(), "AZERLAY_STORE_WORKER="+mode, "AZERLAY_STORE_HOME="+home)
	w.cmd.ExtraFiles = []*os.File{readyWrite, gateRead}
	w.cmd.Stdout = &w.output
	w.cmd.Stderr = &w.output
	t.Cleanup(func() {
		cancel()
		if !w.waited {
			err := w.cmd.Wait()
			if err != nil {
				t.Logf("worker cleanup: %v", err)
			}
		}
		if err := errors.Join(readyRead.Close(), gateWrite.Close()); err != nil {
			t.Error(err)
		}
	})
	if err := w.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(readyWrite.Close(), gateRead.Close()); err != nil {
		t.Fatal(err)
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("missing deadline")
	}
	if err := readyRead.SetReadDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	return w
}
func (w *storeWorker) signal(t *testing.T, want byte) {
	t.Helper()
	var signal [1]byte
	if _, err := io.ReadFull(w.ready, signal[:]); err != nil {
		t.Fatal(err)
	}
	if signal[0] != want {
		t.Fatalf("signal = %q, want %q", signal[0], want)
	}
}
func (w *storeWorker) release(t *testing.T) {
	t.Helper()
	if _, err := w.gate.Write([]byte{'G'}); err != nil {
		t.Fatal(err)
	}
}
func (w *storeWorker) wait(t *testing.T) {
	t.Helper()
	err := w.cmd.Wait()
	w.waited = true
	if err != nil {
		t.Fatalf("worker failed: %v\n%s", err, w.output.String())
	}
}
