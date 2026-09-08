package profilesource

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
)

func TestImportedXDGPaths(t *testing.T) {
	for _, tc := range []struct{ xdg, home, want string }{
		{"/tmp/data", "relative", "/tmp/data"}, {"", "/tmp/home", "/tmp/home/.local/share"},
		{"relative", "/tmp/home", "/tmp/home/.local/share"}, {"", "relative", ""}, {"", "", ""},
	} {
		t.Run(fmt.Sprintf("%q/%q", tc.xdg, tc.home), func(t *testing.T) {
			// Given: environment-only XDG candidates, with no authorized filesystem access.
			t.Setenv("XDG_DATA_HOME", tc.xdg)
			t.Setenv("HOME", tc.home)
			// When: resolve the data parent.
			got, err := ResolveDataHome()
			// Then: only absolute XDG or absolute HOME fallback is accepted.
			if got != tc.want || (err != nil) != (tc.want == "") {
				t.Fatalf("resolution = %q, %v", got, err)
			}
			if err != nil {
				requireStorageError(t, err)
			}
		})
	}
	t.Run("ancestors unchanged", func(t *testing.T) {
		// Given: public XDG ancestors and unrelated data belonging to the caller.
		home := t.TempDir()
		if err := os.Chmod(home, 0755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(home, "unrelated")
		if err := os.WriteFile(path, []byte("keep"), 0644); err != nil {
			t.Fatal(err)
		}
		before := storeTree(t, home)
		// When: import creates only its private app subtree.
		_, err := NewImportedSource(home).Import(context.Background(), storePrepared(t, storeInput), 2, Origin{Kind: "text"})
		// Then: ancestors and unrelated content/modes/inodes are retained.
		if err != nil {
			t.Fatal(err)
		}
		after := storeTree(t, home)
		for path, want := range before {
			if !reflect.DeepEqual(after[path], want) {
				t.Fatalf("ancestor/unrelated changed: %s", path)
			}
		}
	})
}

func TestImportedUnsafePaths(t *testing.T) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(storeInput)))
	paths := []string{"azerlay", "azerlay/sources", "azerlay/profiles", "azerlay/cache", "azerlay/cache/import.lock", "azerlay/cache/source-index.json", "azerlay/sources/" + hash + ".azeron", "azerlay/profiles/" + hash, "azerlay/profiles/" + hash + "/bundle.json", "azerlay/profiles/" + hash + "/p1.json", "azerlay/profiles/" + hash + "/p2.json"}
	for _, path := range paths {
		for _, kind := range []string{"symlink", "mode", "type", "fifo"} {
			t.Run(path+"/"+kind, func(t *testing.T) {
				// Given: a valid committed store, then one unsafe owned path.
				home := t.TempDir()
				s := NewImportedSource(home)
				p := storePrepared(t, storeInput)
				if _, err := s.Import(context.Background(), p, 2, Origin{Kind: "text"}); err != nil {
					t.Fatal(err)
				}
				full := filepath.Join(home, path)
				info, err := os.Lstat(full)
				if err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "mode":
					if err := os.Chmod(full, 0777); err != nil {
						t.Fatal(err)
					}
				case "symlink", "type", "fifo":
					saved := full + ".saved"
					if err := os.Rename(full, saved); err != nil {
						t.Fatal(err)
					}
					switch kind {
					case "symlink":
						if err := os.Symlink(filepath.Base(saved), full); err != nil {
							t.Fatal(err)
						}
					case "fifo":
						if err := syscall.Mkfifo(full, 0600); err != nil {
							t.Fatal(err)
						}
					case "type":
						if info.IsDir() {
							if err := os.WriteFile(full, []byte("blocked"), 0600); err != nil {
								t.Fatal(err)
							}
						} else if err := os.Mkdir(full, 0700); err != nil {
							t.Fatal(err)
						}
					default:
						t.Fatal("unknown fixture kind")
					}
				default:
					t.Fatal("unknown fixture kind")
				}
				before := storeTree(t, home)
				// When: reimport attempts to reuse the unsafe path.
				selection, err := s.Import(context.Background(), p, 1, Origin{Kind: "text"})
				// Then: safe typed failure, no adoption/chmod/following of unknown objects.
				requireStorageError(t, err)
				if selection != (Selection{}) {
					t.Fatal("failure exposed selection")
				}
				if !reflect.DeepEqual(before, storeTree(t, home)) {
					t.Fatal("unsafe path changed store")
				}
			})
		}
	}
	t.Run("owner", func(t *testing.T) {
		// Given: actual file metadata copied with only its owner replaced. chown to
		// another UID is unavailable to the unprivileged test process.
		path := filepath.Join(t.TempDir(), "private")
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			t.Fatal("missing Linux stat")
		}
		foreign := *stat
		foreign.Uid++
		// When: the production ownership boundary receives a foreign owner.
		err = privateInfo(ownerInfo{FileInfo: info, stat: &foreign}, false)
		// Then: exact ownership rejection, without requiring root or changing system users.
		if !errors.Is(err, errUnsafePath) {
			t.Fatalf("owner accepted: %v", err)
		}
	})
}

type ownerInfo struct {
	os.FileInfo
	stat *syscall.Stat_t
}

func (i ownerInfo) Sys() any { return i.stat } // os.FileInfo's stdlib metadata boundary.
