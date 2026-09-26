package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profilesource"
)

func isolateCLI(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	return root
}

func seedCLIStore(t *testing.T, root string) profilesource.Selection {
	t.Helper()
	input := cliRead(t, "../../internal/profileadapter/testdata/bundle.input.json")
	prepared, err := profilesource.PrepareReader(bytes.NewReader(input), profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "PRIVATE_PATH")
	selection, err := profilesource.NewImportedSource(filepath.Join(root, "data")).Import(t.Context(), prepared, 2, profilesource.Origin{Kind: "file", Path: &path})
	if err != nil {
		t.Fatal(err)
	}
	return selection
}

func TestImportPersists(t *testing.T) {
	for _, route := range []string{"file", "stdin", "text"} {
		t.Run(route, func(t *testing.T) {
			// Given: source-order duplicates, raw tokens and bindings absent from CLI DTOs.
			root := isolateCLI(t)
			input := cliRead(t, "../../internal/profileadapter/testdata/bundle.input.json")
			path := filepath.Join(root, "PRIVATE_PATH")
			cliWrite(t, path, input, 0600)
			args := []string{"import", "--json", "--software-release", "2.0.2", "--profile-index", "2"}
			switch route {
			case "file":
				args = append(args, path)
			case "stdin":
				args = append(args, "-")
			case "text":
				args = append(args, "--text", string(input))
			default:
				t.Fatal("unknown fixture route")
			}
			var stdout, stderr bytes.Buffer
			// When
			status := run(args, bytes.NewReader(input), &stdout, &stderr)
			// Then: successful reporting implies complete persisted data and first origin.
			checkCLIStatus(t, cliOutput{status, stdout.String(), stderr.String()}, 0, true)
			assertJSONEqual(t, stdout.String(), cliSuccess("import", cliSelected(savedBundleResult, 2)))
			source := profilesource.NewImportedSource(filepath.Join(root, "data"))
			selected, err := source.Selected(t.Context())
			if err != nil {
				t.Fatalf("successful import did not persist selection: %v", err)
			}
			hash := fmt.Sprintf("%x", sha256.Sum256(input))
			if selected.ProfileIndex != 2 || selected.Source.Hash != hash || !bytes.Equal(cliRead(t, filepath.Join(root, "data/azerlay/sources", hash+".azeron")), input) {
				t.Fatalf("saved identity/ordinal = %+v", selected)
			}
			bundle, err := source.Load(t.Context(), selected.Source)
			if err != nil {
				t.Fatal(err)
			}
			want, err := profilesource.PrepareReader(bytes.NewReader(input), profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export"})
			if err != nil || !reflect.DeepEqual(*bundle, want.Bundle()) {
				t.Fatalf("complete bundle lost: %v", err)
			}
			catalog, err := source.Discover(t.Context())
			if err != nil || len(catalog) != 1 {
				t.Fatalf("catalog=%+v err=%v", catalog, err)
			}
			origin := profilesource.Origin{Kind: route}
			if route == "file" {
				origin.Path = &path
			}
			if !reflect.DeepEqual(catalog[0].Origin, origin) {
				t.Fatalf("origin=%+v want=%+v", catalog[0].Origin, origin)
			}
		})
	}
}

func TestImportSettingsAndProfilesShow(t *testing.T) {
	root := isolateCLI(t)
	input := cliRead(t, "../../internal/profileadapter/testdata/v1-settings.input.json")
	var stdout, stderr bytes.Buffer
	status := run([]string{"import", "--json", "--software-release", "2.0.2", "--text", string(input)}, &observedReader{}, &stdout, &stderr)
	if status != 0 || stderr.Len() != 0 {
		t.Fatalf("settings import status=%d stderr=%q", status, stderr.String())
	}
	assertJSONEqual(t, stdout.String(), cliSuccess("import", cliSelected(settingsResult, 1)))
	before := cliTree(t, root)
	stdout.Reset()
	status = run([]string{"profiles", "show", "--json"}, &observedReader{}, &stdout, &stderr)
	if status != 0 || stderr.Len() != 0 {
		t.Fatalf("settings show status=%d stderr=%q", status, stderr.String())
	}
	assertJSONEqual(t, stdout.String(), cliSuccess("profiles show", cliSelected(settingsResult, 1)))
	if !reflect.DeepEqual(before, cliTree(t, root)) {
		t.Fatal("settings show changed disk")
	}
	selected, err := profilesource.NewImportedSource(filepath.Join(root, "data")).Selected(t.Context())
	if err != nil || selected.ProfileIndex != 1 {
		t.Fatalf("settings selection = %+v, %v", selected, err)
	}
}

func TestProfilesShow(t *testing.T) {
	for _, jsonMode := range []bool{false, true} {
		t.Run(fmt.Sprint(jsonMode), func(t *testing.T) {
			// Given: a committed selection from the real storage API, not CLI dispatch.
			root := isolateCLI(t)
			seedCLIStore(t, root)
			before := cliTree(t, root)
			reader := &observedReader{}
			var stdout, stderr bytes.Buffer
			args := []string{"profiles", "show"}
			if jsonMode {
				args = append(args, "--json")
			}
			// When: no source, release or selector is supplied.
			status := run(args, reader, &stdout, &stderr)
			// Then: the entire safe projection uses the stored ordinal, without writes.
			got := cliOutput{status, stdout.String(), stderr.String()}
			checkCLIStatus(t, got, 0, true)
			checkCLIPrivate(t, got)
			want := cliSelected(savedBundleResult, 2)
			if jsonMode {
				assertJSONEqual(t, got.stdout, cliSuccess("profiles show", want))
			} else {
				checkCLIHuman(t, got.stdout, want)
			}
			if reader.reads != 0 || !reflect.DeepEqual(before, cliTree(t, root)) {
				t.Fatal("show read input or changed storage")
			}
		})
	}
}

func TestProfilesShowFailures(t *testing.T) {
	for _, damage := range []string{"empty", "index", "original", "resolution"} {
		t.Run(damage, func(t *testing.T) {
			// Given: no selection or real corrupt storage, never a mock source.
			root := isolateCLI(t)
			code, stage := "ERR_PROFILE_STORAGE", "storage"
			switch damage {
			case "empty":
				code, stage = "ERR_PROFILE_NOT_FOUND", "selection"
			case "index", "original":
				selection := seedCLIStore(t, root)
				path := filepath.Join(root, "data/azerlay/cache/source-index.json")
				if damage == "original" {
					path = filepath.Join(root, "data/azerlay/sources", selection.Source.Hash+".azeron")
				}
				cliWrite(t, path, []byte("{"), 0600)
			case "resolution":
				t.Setenv("HOME", "")
				t.Setenv("XDG_DATA_HOME", "relative")
			default:
				t.Fatal("unknown fixture damage")
			}
			before := cliTree(t, root)
			var stdout, stderr bytes.Buffer
			// When
			status := run([]string{"profiles", "show", "--json"}, &observedReader{}, &stdout, &stderr)
			// Then: no partial result, reset, repair or private error cause.
			checkCLIFailure(t, cliOutput{status, stdout.String(), stderr.String()}, 1, "profiles show", code, stage, "null")
			checkCLIPrivate(t, cliOutput{status, stdout.String(), stderr.String()})
			if !reflect.DeepEqual(before, cliTree(t, root)) {
				t.Fatal("failed show changed storage")
			}
		})
	}
}

func TestImportFailureBeforeStorage(t *testing.T) {
	// Given: storage resolution cannot succeed, but earlier boundaries must win.
	root := isolateCLI(t)
	t.Setenv("HOME", "")
	t.Setenv("XDG_DATA_HOME", "relative")
	for _, tc := range []struct{ text, release, code, stage, result string }{
		{"{", "2.0.2", "ERR_IMPORT_JSON", "decode", "null"},
		{`{"id":[],"inputs":[]}`, "2.0.2", "ERR_IMPORT_ROOT", "raw", "null"},
		{fixtureText, "2.0.3", "ERR_IMPORT_UNSUPPORTED_VERSION", "normalize", "null"},
		{cliBundle, "2.0.2", "ERR_PROFILE_SELECTION_REQUIRED", "selection", cliBundleResult},
		{`{"profiles":[]}`, "2.0.2", "ERR_PROFILE_NOT_FOUND", "selection", cliEmptyResult},
	} {
		t.Run(tc.code, func(t *testing.T) {
			before := cliTree(t, root)
			var stdout, stderr bytes.Buffer
			// When
			status := run([]string{"import", "--json", "--software-release", tc.release, "--text", tc.text}, &observedReader{}, &stdout, &stderr)
			// Then: no storage error replaces the actual input or selection failure.
			checkCLIFailure(t, cliOutput{status, stdout.String(), stderr.String()}, 1, "import", tc.code, tc.stage, tc.result)
			if !reflect.DeepEqual(before, cliTree(t, root)) {
				t.Fatal("rejected import changed storage")
			}
		})
	}
}
