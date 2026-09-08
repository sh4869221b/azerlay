package profilesource

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
)

func TestImportedSourceFacade(t *testing.T) {
	// Given: one committed source and an unindexed hash-shaped directory.
	home, selection, _ := loadFixture(t)
	if err := os.Mkdir(filepath.Join(home, "azerlay/profiles", strings.Repeat("a", 64)), 0700); err != nil {
		t.Fatal(err)
	}
	before := storeTree(t, home)
	var source ProfileSource = NewImportedSource(home)
	// When: discover through the public provider interface.
	got, err := source.Discover(context.Background())
	// Then: only the committed record is exposed and the provider identity is stable.
	if err != nil {
		t.Fatal(err)
	}
	if source.ID() != "imported" || len(got) != 1 || got[0].Ref != selection.Source || got[0].ProfileCount != 2 || got[0].Source.SoftwareRelease != "2.0.2" {
		t.Fatalf("descriptors = %+v", got)
	}
	if !reflect.DeepEqual(before, storeTree(t, home)) {
		t.Fatal("discover wrote store")
	}
}

func TestImportedWatch(t *testing.T) {
	// Given: a committed immutable ref.
	home, selection, _ := loadFixture(t)
	before := storeTree(t, home)
	// When: watch it without creating a subscription worker.
	changes, err := NewImportedSource(home).Watch(context.Background(), selection.Source)
	// Then: the channel is already closed, not eventually closed.
	if err != nil {
		t.Fatal(err)
	}
	select {
	case _, open := <-changes:
		if open {
			t.Fatal("watch emitted a change")
		}
	default:
		t.Fatal("watch channel is not immediately closed")
	}
	if !reflect.DeepEqual(before, storeTree(t, home)) {
		t.Fatal("watch wrote store")
	}
}

func TestImportedReadReferences(t *testing.T) {
	for _, hash := range []string{strings.Repeat("a", 64), "", "../bad", strings.Repeat("A", 64)} {
		for _, operation := range []string{"load", "watch"} {
			t.Run(operation+"/"+hash, func(t *testing.T) {
				// Given: an unknown or malformed reference.
				home, _, _ := loadFixture(t)
				before := storeTree(t, home)
				// When: access the reference.
				err := readOperation(t, NewImportedSource(home), readRequest{ctx: context.Background(), operation: operation, ref: SourceRef{Hash: hash}})
				// Then: invalid syntax is storage failure; unindexed valid syntax is absent.
				want := ERR_PROFILE_STORAGE
				if hash == strings.Repeat("a", 64) {
					want = ERR_PROFILE_NOT_FOUND
				}
				requireReadError(t, err, want)
				if !reflect.DeepEqual(before, storeTree(t, home)) {
					t.Fatal("reference failure wrote store")
				}
			})
		}
	}
}

func TestImportedReadNoWrite(t *testing.T) {
	for _, state := range []string{"absent parent", "absent app", "absent index", "corrupt index", "canceled", "relative", "empty", "file parent"} {
		for _, operation := range []string{"discover", "selected", "load", "watch"} {
			t.Run(state+"/"+operation, func(t *testing.T) {
				// Given: missing or unusable storage, with a complete before-tree.
				home := t.TempDir()
				data := home
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				switch state {
				case "absent parent":
					data = filepath.Join(home, "missing", "data")
				case "absent app":
				case "absent index", "corrupt index":
					if err := os.MkdirAll(filepath.Join(home, "azerlay/cache"), 0700); err != nil {
						t.Fatal(err)
					}
					if state == "corrupt index" {
						writeLoadFile(t, filepath.Join(home, "azerlay/cache/source-index.json"), []byte("{"))
					}
				case "canceled":
					cancel()
				case "relative":
					data = "relative"
				case "empty":
					data = ""
				case "file parent":
					data = filepath.Join(home, "blocked")
					writeLoadFile(t, data, []byte("private"))
				default:
					t.Fatal("unknown state")
				}
				before := storeTree(t, home)
				// When: construct lazily and invoke one read surface.
				err := readOperation(t, NewImportedSource(data), readRequest{ctx: ctx, operation: operation, ref: SourceRef{Hash: strings.Repeat("a", 64)}})
				// Then: missing data is empty/absent, all other failures are safe; no writes.
				switch state {
				case "absent parent", "absent app", "absent index":
					if operation == "discover" {
						if err != nil {
							t.Fatal(err)
						}
					} else {
						requireReadError(t, err, ERR_PROFILE_NOT_FOUND)
					}
				default:
					requireStorageError(t, err)
				}
				if state == "canceled" && !errors.Is(err, context.Canceled) {
					t.Fatal("cancellation cause lost")
				}
				if !reflect.DeepEqual(before, storeTree(t, home)) {
					t.Fatal("read created or changed paths")
				}
			})
		}
	}
}

func TestImportedReadUnsafePaths(t *testing.T) {
	for _, relative := range []string{"azerlay", "azerlay/cache", "azerlay/cache/source-index.json", "azerlay/sources", "azerlay/profiles", "azerlay/sources/HASH.azeron", "azerlay/profiles/HASH", "azerlay/profiles/HASH/bundle.json", "azerlay/profiles/HASH/p1.json", "azerlay/profiles/HASH/p2.json"} {
		for _, damage := range []string{"mode", "symlink", "type", "fifo"} {
			t.Run(relative+"/"+damage, func(t *testing.T) {
				// Given: an unsafe app-owned component (including in-root symlinks).
				home, selection, _ := loadFixture(t)
				path := filepath.Join(home, strings.ReplaceAll(relative, "HASH", selection.Source.Hash))
				switch damage {
				case "mode":
					if err := os.Chmod(path, 0777); err != nil {
						t.Fatal(err)
					}
				case "symlink", "type", "fifo":
					info, err := os.Stat(path)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.Rename(path, path+".saved"); err != nil {
						t.Fatal(err)
					}
					switch damage {
					case "symlink":
						if err := os.Symlink(filepath.Base(path)+".saved", path); err != nil {
							t.Fatal(err)
						}
					case "fifo":
						if err := syscall.Mkfifo(path, 0600); err != nil {
							t.Fatal(err)
						}
					case "type":
						if info.IsDir() {
							writeLoadFile(t, path, []byte("blocked"))
						} else if err := os.Mkdir(path, 0700); err != nil {
							t.Fatal(err)
						}
					default:
						t.Fatal("unknown damage")
					}
				default:
					t.Fatal("unknown damage")
				}
				before := storeTree(t, home)
				// When: load the committed original and derived set.
				got, err := NewImportedSource(home).Load(context.Background(), selection.Source)
				// Then: no symlink following, chmod, blocking FIFO read, or partial success.
				requireStorageError(t, err)
				if got != nil || !reflect.DeepEqual(before, storeTree(t, home)) {
					t.Fatal("unsafe read changed state or returned model")
				}
			})
		}
	}
}

type readRequest struct {
	ctx       context.Context
	operation string
	ref       SourceRef
}

func readOperation(t *testing.T, source *ImportedSource, request readRequest) error {
	t.Helper()
	switch request.operation {
	case "discover":
		got, err := source.Discover(request.ctx)
		if len(got) != 0 {
			t.Fatal("unexpected descriptors")
		}
		return err
	case "selected":
		got, err := source.Selected(request.ctx)
		if got != (Selection{}) {
			t.Fatal("unexpected selection")
		}
		return err
	case "load":
		got, err := source.Load(request.ctx, request.ref)
		if got != nil {
			t.Fatal("unexpected model")
		}
		return err
	case "watch":
		got, err := source.Watch(request.ctx, request.ref)
		if got != nil {
			t.Fatal("unexpected watch")
		}
		return err
	default:
		t.Fatal("unknown operation")
		return nil
	}
}

func requireReadError(t *testing.T, err error, code string) {
	t.Helper()
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != code || err.Error() != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
	if code == ERR_PROFILE_STORAGE {
		requireStorageError(t, err)
	}
}
