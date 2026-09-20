package profilesource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
	"github.com/sh4869221b/azerlay/internal/profiledecode"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

// allow: SIZE_OK - Task 4 confines full recovery, corruption and ownership
// regression coverage to load_test.go; no additional test-file scope is authorized.

func loadFixture(t *testing.T) (string, Selection, profile.ProfileBundle) {
	t.Helper()
	home := t.TempDir()
	input := storeBytes(t, "../profileadapter/testdata/bundle.input.json")
	p := storePrepared(t, string(input))
	path := filepath.Join(home, "removed-input.json")
	writeLoadFile(t, path, input)
	selection, err := NewImportedSource(home).Import(context.Background(), p, 2, Origin{Kind: "file", Path: &path})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	return home, selection, p.Bundle()
}

func TestImportedLoadRestart(t *testing.T) {
	// Given: a persisted bundle whose provenance path is absent.
	home, wantSelection, want := loadFixture(t)
	source := NewImportedSource(home)
	// When: a new instance resolves and loads its saved selection.
	selection, err := source.Selected(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got, err := source.Load(context.Background(), selection.Source)
	// Then: the full model and explicit ordinal survive restart.
	if err != nil || selection != wantSelection || got == nil || !reflect.DeepEqual(*got, want) {
		t.Fatalf("restart = %+v, %v", selection, err)
	}
}

func TestImportedLoadCacheMiss(t *testing.T) {
	for _, member := range []string{"bundle.json", "p1.json", "p2.json"} {
		for _, damage := range []string{"missing", "corrupt", "stale", "mismatched model"} {
			t.Run(member+"/"+damage, func(t *testing.T) {
				// Given: exactly one missing, corrupt, or incompatible derived member.
				home, selection, want := loadFixture(t)
				path := filepath.Join(home, "azerlay/profiles", selection.Source.Hash, member)
				switch damage {
				case "missing":
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				case "corrupt":
					if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
						t.Fatal(err)
					}
				case "stale":
					data := mutateStorage(t, storeBytes(t, path), storageMutation{"normalizer_version", `"old"`})
					writeLoadFile(t, path, data)
				case "mismatched model":
					data := storeBytes(t, path)
					data = bytes.ReplaceAll(data, []byte(`"id":"duplicate-id"`), []byte(`"id":"different-id"`))
					writeLoadFile(t, path, data)
				default:
					t.Fatal("unknown fixture")
				}
				before := storeTree(t, home)
				// When: load the committed source.
				got, err := NewImportedSource(home).Load(context.Background(), selection.Source)
				// Then: full reconstruction, with no cache repair or tree mutation.
				if err != nil || got == nil || !reflect.DeepEqual(*got, want) {
					t.Fatalf("recovery = %v", err)
				}
				assertLoadValues(t, *got)
				if !reflect.DeepEqual(before, storeTree(t, home)) {
					t.Fatal("load repaired store")
				}
			})
		}
	}
}

func writeLoadFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func assertLoadValues(t *testing.T, got profile.ProfileBundle) {
	t.Helper()
	if got.SchemaVersion != 1 || got.RootKind != profile.RootBundle || len(got.Profiles) != 2 {
		t.Fatal("bundle shape")
	}
	first, second := got.Profiles[0], got.Profiles[1]
	if first.ID == nil || second.ID == nil || *first.ID != "duplicate-id" || *second.ID != "duplicate-id" || first.Name == nil || *first.Name != "Primary" || second.Name != nil || len(first.Controls) != 2 || len(second.Controls) != 1 {
		t.Fatal("profile values/order")
	}
	if string(got.Raw.Version) != "1e+09" || string(first.Raw.Version) != "-0" || second.Raw.Version != nil || string(second.Raw.Name) != "null" {
		t.Fatal("raw nil/null/number spelling")
	}
	control := first.Controls[0]
	if string(control.Raw.Fields["opaque"]) != "null" || string(control.Raw.Fields["spelling"]) != "9e2" {
		t.Fatal("binding raw tokens")
	}
	long, double := control.Bindings[1], control.Bindings[2]
	if long.Kind != profile.BindingKeyboard || long.TriggerDelayMS == nil || *long.TriggerDelayMS != 500 || long.Actions[0].Code != profile.KEY_U || long.ReleaseBehavior == nil || *long.ReleaseBehavior != "regular" {
		t.Fatal("long binding")
	}
	if double.TriggerIntervalMS == nil || *double.TriggerIntervalMS != 150 || len(double.Actions) != 2 || double.Actions[0].Code != profile.KEY_P || double.Actions[1].Code != profile.KEY_L {
		t.Fatal("double binding")
	}
}

func TestImportedLoadOwnership(t *testing.T) {
	for _, recovery := range []bool{false, true} {
		t.Run(fmt.Sprint(recovery), func(t *testing.T) {
			// Given: caller-mutated returned pointers, slices, maps and opaque byte tokens.
			home, selection, want := loadFixture(t)
			if recovery {
				if err := os.Remove(filepath.Join(home, "azerlay/profiles", selection.Source.Hash, "p2.json")); err != nil {
					t.Fatal(err)
				}
			}
			s := NewImportedSource(home)
			first, err := s.Load(context.Background(), selection.Source)
			if err != nil {
				t.Fatal(err)
			}
			first.Raw.Version[0] = '9'
			first.Raw.Unknown["bundleFlag"][0] = 'x'
			*first.Profiles[0].Name = "mutated"
			first.Profiles[0].Controls[0].Bindings[1].Actions[0].Code = profile.KEY_I
			*first.Profiles[0].Controls[0].Bindings[1].TriggerDelayMS = 7
			first.Profiles[0].Controls[0].Raw.Fields["opaque"][0] = 'x'
			descriptors, err := s.Discover(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			*descriptors[0].Origin.Path = "mutated"
			descriptors[0].Source.SoftwareRelease = "9.9.9"
			// When: call the same source instance again.
			got, err := s.Load(context.Background(), selection.Source)
			again, discoverErr := s.Discover(context.Background())
			// Then: no earlier returned allocation influences later results.
			if err != nil || got == nil || !reflect.DeepEqual(*got, want) || discoverErr != nil || *again[0].Origin.Path != filepath.Join(home, "removed-input.json") || again[0].Source.SoftwareRelease != "2.0.2" {
				t.Fatal("caller mutation escaped ownership")
			}
		})
	}
}

func TestImportedLoadCorruption(t *testing.T) {
	for _, damage := range []string{"original missing", "original damaged", "original damaged cache absent", "release", "scope", "count", "export token", "index malformed", "index ordinal", "index duplicate", "index oversized", "original oversized"} {
		t.Run(damage, func(t *testing.T) {
			// Given: corruption of committed authority, not replaceable derived data.
			home, selection, _ := loadFixture(t)
			indexPath := filepath.Join(home, "azerlay/cache/source-index.json")
			originalPath := filepath.Join(home, "azerlay/sources", selection.Source.Hash+".azeron")
			index := storeIndex(t, home)
			switch damage {
			case "original missing":
				if err := os.Remove(originalPath); err != nil {
					t.Fatal(err)
				}
			case "original damaged", "original damaged cache absent":
				writeLoadFile(t, originalPath, []byte("broken"))
				if damage == "original damaged cache absent" {
					if err := os.Remove(filepath.Join(home, "azerlay/profiles", selection.Source.Hash, "bundle.json")); err != nil {
						t.Fatal(err)
					}
				}
			case "release":
				index.Sources[0].SoftwareRelease = "2.0.3"
			case "scope":
				index.Sources[0].SourceScope = "file"
			case "count":
				index.Sources[0].ProfileCount = 3
			case "export token":
				token := "null"
				index.Sources[0].ExportVersion = &token
			case "index ordinal":
				index.Selected.ProfileIndex = 3
			case "index duplicate":
				index.Sources = append(index.Sources, index.Sources[0])
			case "index malformed":
				writeLoadFile(t, indexPath, []byte("{"))
			case "index oversized":
				writeLoadFile(t, indexPath, []byte(strings.Repeat(" ", maxIndexBytes+1)))
			case "original oversized":
				writeLoadFile(t, originalPath, []byte(strings.Repeat(" ", (16<<20)+1)))
			default:
				t.Fatal("unknown damage")
			}
			switch damage {
			case "release", "scope", "count", "export token", "index ordinal", "index duplicate":
				data, err := json.Marshal(index)
				if err != nil {
					t.Fatal(err)
				}
				writeLoadFile(t, indexPath, data)
			}
			before := storeTree(t, home)
			// When: load, even with otherwise valid caches.
			got, err := NewImportedSource(home).Load(context.Background(), selection.Source)
			// Then: safe storage error, retained cause, nil model and unchanged disk.
			requireStorageError(t, err)
			if got != nil || !reflect.DeepEqual(before, storeTree(t, home)) {
				t.Fatal("corruption returned model or wrote state")
			}
			if damage == "release" || damage == "scope" {
				var cause *profileadapter.NormalizeError
				if !errors.As(err, &cause) {
					t.Fatal("unsupported cause lost")
				}
			}
		})
	}
}

func TestImportedLoadRecoveryAdmission(t *testing.T) {
	for _, input := range []string{"{", `{"profiles":[{"inputs":null}]}`, macroExport(1001)} {
		t.Run(fmt.Sprint(len(input)), func(t *testing.T) {
			// Given: a syntactically valid index pointing to hash-matching but inadmissible bytes.
			home, _, _ := loadFixture(t)
			index := storeIndex(t, home)
			hash := fmt.Sprintf("%x", sha256.Sum256([]byte(input)))
			index.Sources[0].SourceHash = hash
			index.Selected.SourceHash = hash
			data, err := encodeIndex(index)
			if err != nil {
				t.Fatal(err)
			}
			writeLoadFile(t, filepath.Join(home, "azerlay/cache/source-index.json"), data)
			writeLoadFile(t, filepath.Join(home, "azerlay/sources", hash+".azeron"), []byte(input))
			before := storeTree(t, home)
			// When: reconstruct the missing cache through all original admission stages.
			got, err := NewImportedSource(home).Load(context.Background(), SourceRef{Hash: hash})
			// Then: boundary causes survive the stable storage wrapper, no partial bundle.
			requireStorageError(t, err)
			var decode *profiledecode.DecodeError
			var parse *profileraw.ParseError
			var normalize *profileadapter.NormalizeError
			if !errors.As(err, &decode) && !errors.As(err, &parse) && !errors.As(err, &normalize) {
				t.Fatal("admission cause lost")
			}
			if got != nil || !reflect.DeepEqual(before, storeTree(t, home)) {
				t.Fatal("admission failure changed state")
			}
		})
	}
}

func TestImportedLoadReaderRecovery(t *testing.T) {
	// Given: raw binary LZMA captured as reader; interpreting these same bytes as
	// text would fail, so reconstruction must honor the first binary input kind.
	home := t.TempDir()
	input := storeBytes(t, "../profileadapter/testdata/bundle.input.json")
	want := storePrepared(t, string(input)).Bundle()
	p, err := PrepareReader(bytes.NewReader(compressedExport(t, string(input))), want.Source)
	if err != nil {
		t.Fatal(err)
	}
	s := NewImportedSource(home)
	selection, err := s.Import(context.Background(), p, 2, Origin{Kind: "stdin"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(home, "azerlay/profiles")); err != nil {
		t.Fatal(err)
	}
	before := storeTree(t, home)
	// When: load with all derived directories absent.
	got, err := s.Load(context.Background(), selection.Source)
	// Then: exact first-route admission and full reconstruction without mkdir.
	if err != nil || got == nil || !reflect.DeepEqual(*got, want) || !reflect.DeepEqual(before, storeTree(t, home)) {
		t.Fatalf("reader recovery = %v", err)
	}
}

func TestImportedLoadCacheAuthority(t *testing.T) {
	// Given: a complete compatible cache set whose legal normalized name differs
	// from the original, plus historical index interpretation revisions.
	home, selection, want := loadFixture(t)
	name := "Cached interpretation"
	want.Profiles[0].Name = &name
	index := storeIndex(t, home)
	index.Sources[0].DecoderVersion = "historical"
	index.Sources[0].NormalizerVersion = "historical"
	index.Sources[0].ModelSchemaVersion = 7
	data, err := encodeIndex(index)
	if err != nil {
		t.Fatal(err)
	}
	writeLoadFile(t, filepath.Join(home, "azerlay/cache/source-index.json"), data)
	artifacts, err := encodeArtifacts(index.Sources[0], want)
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range artifacts {
		writeLoadFile(t, filepath.Join(home, "azerlay", artifact.path), artifact.data)
	}
	before := storeTree(t, home)
	// When: load a current full set without treating historical index revisions as envelopes.
	got, err := NewImportedSource(home).Load(context.Background(), selection.Source)
	// Then: the bundle cache is authority, not an unconditional re-normalization.
	if err != nil || got == nil || !reflect.DeepEqual(*got, want) || !reflect.DeepEqual(before, storeTree(t, home)) {
		t.Fatalf("cache authority = %v", err)
	}
}

func TestImportedLoadOversizedCache(t *testing.T) {
	for _, member := range []string{"bundle.json", "p1.json", "p2.json"} {
		t.Run(member, func(t *testing.T) {
			// Given: a real sparse oversized member; retain bytes outside it separately
			// rather than allocating 512 MiB just for the tree assertion.
			home, selection, want := loadFixture(t)
			path := filepath.Join(home, "azerlay/profiles", selection.Source.Hash, member)
			before := storeTree(t, home)
			if err := os.Truncate(path, maxCacheBytes+1); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			// When: load using the production 512 MiB boundary.
			got, err := NewImportedSource(home).Load(context.Background(), selection.Source)
			// Then: full memory-only recovery; the sparse file itself was not repaired.
			if err != nil || got == nil || !reflect.DeepEqual(*got, want) {
				t.Fatalf("oversize recovery = %v", err)
			}
			after, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Size() != after.Size() || info.Mode() != after.Mode() || info.ModTime() != after.ModTime() || !os.SameFile(info, after) {
				t.Fatal("oversize cache changed")
			}
			// Restore only the fixture's sparse extension to compare every original byte.
			rel, err := filepath.Rel(home, path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Truncate(path, int64(len(before[rel].Bytes))); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, storeTree(t, home)) {
				t.Fatal("oversize load changed tree")
			}
		})
	}
}
