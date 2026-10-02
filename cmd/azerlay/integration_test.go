package main

// allow: SIZE_OK - Existing binary regression suite; task 5 may only adapt
// successful-import expectations. Task 6 owns additions, not a suite split.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/profiledecode"
	"github.com/ulikunitz/xz/lzma"
)

var cliBinary string

// The child is built from this checkout once, not taken from PATH or a stale
// artifact. Returning through cliTestMain ensures cleanup precedes os.Exit.
func TestMain(m *testing.M) {
	if mode := os.Getenv("AZERLAY_RUN_NATIVE_CHILD"); mode != "" {
		os.Exit(runNativeChild(mode))
	}
	if os.Getenv("AZERLAY_TEST_WAYLAND_DISPLAY") == "" {
		launcher, err := filepath.Abs("../../scripts/test-wayland.sh")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		cmd := exec.Command("bash", append([]string{launcher, os.Args[0]}, os.Args[1:]...)...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "Wayland fixture:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(cliTestMain(m))
}

func cliTestMain(m *testing.M) (status int) {
	dir, err := os.MkdirTemp("", "azerlay-cli-tests-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintln(os.Stderr, "integration cleanup:", err)
			status = 1
		} else if _, err := os.Lstat(dir); !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stderr, "integration directory still exists:", dir, err)
			status = 1
		} else {
			fmt.Fprintln(os.Stderr, "integration binary removed:", dir)
		}
	}()
	cliBinary = filepath.Join(dir, "azerlay")
	// Cache measurement probe: ordinary CLI builds retain this 30-second bound.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", cliBinary, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build CLI: %v\n%s", err, output)
		return 1
	}
	return m.Run()
}

type cliOutput struct {
	status         int
	stdout, stderr string
}

func cliEnvironment(root string) []string {
	// Do not inherit the user's HOME or any XDG path, even in tests unrelated
	// to persistence. The Go build above retains its ordinary build cache.
	env := make([]string, 0)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if key != "HOME" && !strings.HasPrefix(key, "XDG_") {
			env = append(env, entry)
		}
	}
	for _, key := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "XDG_RUNTIME_DIR"} {
		env = append(env, key+"="+filepath.Join(root, key))
	}
	return env
}

func invokeCLI(t *testing.T, env []string, stdin []byte, args ...string) cliOutput {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, cliBinary, args...)
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(stdin) // os/exec connects this reader to a real pipe.
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("CLI did not complete within 30 seconds: %v", ctx.Err())
	}
	status := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("execute CLI: %v", err)
		}
		status = exit.ExitCode()
	}
	return cliOutput{status, stdout.String(), stderr.String()}
}

func checkCLIStatus(t *testing.T, got cliOutput, status int, jsonMode bool) {
	t.Helper()
	if got.status != status || (jsonMode && got.stderr != "") {
		t.Fatalf("CLI status=%d want=%d stdout=%q stderr=%q", got.status, status, got.stdout, got.stderr)
	}
}

func cliSuccess(command, result string) string {
	return fmt.Sprintf(`{"schema_version":1,"command":%q,"ok":true,"result":%s,"error":null}`, command, result)
}

// Compare every machine field, but never pin explanatory prose. parsedJSON
// uses json.Number and rejects a second object or trailing non-JSON output.
func checkCLIFailure(t *testing.T, got cliOutput, status int, command, code, stage, result string) {
	t.Helper()
	checkCLIStatus(t, got, status, true)
	value := parsedJSON(t, got.stdout)
	envelope, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("failure is not an object: %s", got.stdout)
	}
	failure, ok := envelope["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error object: %s", got.stdout)
	}
	for _, field := range []string{"summary", "remediation"} {
		if text, ok := failure[field].(string); !ok || text == "" {
			t.Fatalf("missing error %s: %s", field, got.stdout)
		}
		delete(failure, field)
	}
	want := fmt.Sprintf(`{"schema_version":1,"command":%q,"ok":false,"result":%s,"error":{"code":%q,"stage":%q}}`, command, result, code, stage)
	if !strings.HasSuffix(got.stdout, "\n") || !reflect.DeepEqual(value, parsedJSON(t, want)) {
		t.Fatalf("failure machine fields=%v want=%s", value, want)
	}
}

func cliRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func cliWrite(t *testing.T, path string, data []byte, mode fs.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func cliCompressed(t *testing.T, text string) []byte {
	t.Helper()
	header := []byte{0xdb, 0, 0, 0, 0} // MessagePack str32, not a map or bin.
	binary.BigEndian.PutUint32(header[1:], uint32(len(text)))
	var output bytes.Buffer
	writer, err := (lzma.WriterConfig{DictCap: lzma.MinDictCap, EOSMarker: true}).NewWriter(&output)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(append(header, text...)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

const cliSingle = `{"id":"synthetic","inputs":[]}`
const cliSingleResult = `{"root_kind":"single","export_version":{"present":false,"value":null},"profiles":[{"index":1,"name":null,"input_count":0}],"selected_profile_index":null,"warnings":[]}`
const cliBundle = `{"profiles":[{"id":"same","name":"duplicate","inputs":[]},{"id":"same","name":"","inputs":[]},{"inputs":[]}]}`
const cliBundleResult = `{"root_kind":"bundle","export_version":{"present":false,"value":null},"profiles":[{"index":1,"name":"duplicate","input_count":0},{"index":2,"name":"","input_count":0},{"index":3,"name":null,"input_count":0}],"selected_profile_index":null,"warnings":[]}`
const cliEmptyResult = `{"root_kind":"bundle","export_version":{"present":false,"value":null},"profiles":[],"selected_profile_index":null,"warnings":[]}`

func cliSelected(result string, index int) string {
	return strings.Replace(result, `"selected_profile_index":null`, fmt.Sprintf(`"selected_profile_index":%d`, index), 1)
}

func TestCLIInputForms(t *testing.T) {
	t.Parallel()
	for _, fixture := range []struct{ name, text, result string }{
		{"single", cliSingle, cliSingleResult}, {"duplicate_bundle", cliBundle, cliBundleResult},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			env := cliEnvironment(t.TempDir())
			// Choose padding by data, not timing. Require different alphabets and
			// an actual '=' so all four base64 forms are discriminatory.
			var compressed []byte
			for spaces := 0; spaces < 64; spaces++ {
				compressed = cliCompressed(t, fixture.text+strings.Repeat(" ", spaces))
				encoded := base64.StdEncoding.EncodeToString(compressed)
				if strings.HasSuffix(encoded, "=") && strings.ContainsAny(encoded, "+/") {
					break
				}
			}
			encoded := base64.StdEncoding.EncodeToString(compressed)
			if !strings.HasSuffix(encoded, "=") || !strings.ContainsAny(encoded, "+/") {
				t.Fatal("synthetic envelope does not distinguish padding and alphabets")
			}
			forms := []struct {
				name string
				data []byte
				text bool
			}{
				{"json", []byte(fixture.text), true}, {"lzma_alone", compressed, false},
				{"url_padded", []byte(base64.URLEncoding.EncodeToString(compressed)), true},
				{"url_unpadded", []byte(base64.RawURLEncoding.EncodeToString(compressed)), true},
				{"std_padded", []byte(base64.StdEncoding.EncodeToString(compressed)), true},
				{"std_unpadded", []byte(base64.RawStdEncoding.EncodeToString(compressed)), true},
			}
			for _, form := range forms {
				t.Run(form.name, func(t *testing.T) {
					path := filepath.Join(t.TempDir(), "export")
					cliWrite(t, path, form.data, 0o400)
					for _, command := range []string{"validate", "import"} {
						args := []string{command, "--software-release", "2.0.2", "--json"}
						result := fixture.result
						if command == "import" {
							index := 1
							if fixture.name == "duplicate_bundle" {
								index = 2
								args = append(args, "--profile-index=2")
							}
							result = cliSelected(result, index)
						}
						for _, route := range []string{"file", "stdin", "text"} {
							if route == "text" && !form.text {
								continue // Binary LZMA is reader/file input, not argv text.
							}
							t.Run(command+"/"+route, func(t *testing.T) {
								source := []string{path}
								var stdin []byte
								if route == "stdin" {
									source, stdin = []string{"-"}, form.data
								} else if route == "text" {
									source = []string{"--text", string(form.data)}
								}
								got := invokeCLI(t, env, stdin, append(append([]string{}, args...), source...)...)
								checkCLIStatus(t, got, 0, true)
								assertJSONEqual(t, got.stdout, cliSuccess(command, result))
							})
						}
					}
				})
			}
		})
	}
	t.Run("corrupt_lzma_is_incomplete_decompression", func(t *testing.T) {
		text := strings.TrimSpace(string(cliRead(t, "testdata/cli/corrupt-lzma.txt")))
		compressed, err := base64.RawURLEncoding.DecodeString(text)
		if err != nil || len(compressed) <= 13 {
			t.Fatalf("fixture base64/header: %v, %d bytes", err, len(compressed))
		}
		reader, err := lzma.NewReader(bytes.NewReader(compressed))
		if err != nil {
			t.Fatalf("fixture must have valid LZMA header: %v", err)
		}
		if _, err := io.ReadAll(reader); err == nil {
			t.Fatal("fixture decompression completed; want truncated body")
		}
		document, err := profiledecode.DecodeText(text)
		var decodeError *profiledecode.DecodeError
		if !errors.As(err, &decodeError) || decodeError.Code != profiledecode.ERR_IMPORT_LZMA_CORRUPT || !reflect.DeepEqual(document, profiledecode.Document{}) {
			t.Fatalf("DecodeText fixture=%#v error=%v; want ERR_IMPORT_LZMA_CORRUPT and zero document", document, err)
		}
		got := invokeCLI(t, cliEnvironment(t.TempDir()), nil, "validate", "--json", "--software-release", "2.0.2", "testdata/cli/corrupt-lzma.txt")
		checkCLIFailure(t, got, 1, "validate", "ERR_IMPORT_LZMA_CORRUPT", "decode", "null")
	})
}

const cliPrivateResult = `{"root_kind":"single","export_version":{"present":true,"value":"ALLOWED_VERSION\n\u001b"},"profiles":[{"index":1,"name":"ALLOWED_NAME\n\u001b","input_count":1}],"selected_profile_index":null,"warnings":[{"code":"WARN_IMPORT_UNKNOWN_BINDINGS","profile_index":1,"count":3}]}`

const cliPersistedResult = `{"root_kind":"bundle","export_version":{"present":true,"value":1e+09},"profiles":[{"index":1,"name":"Primary","input_count":2},{"index":2,"name":null,"input_count":1}],"selected_profile_index":null,"warnings":[{"code":"WARN_IMPORT_UNKNOWN_BINDINGS","profile_index":1,"count":2},{"code":"WARN_IMPORT_UNKNOWN_BINDINGS","profile_index":2,"count":3}]}`

// Seed through the real file route, then remove that route before any restart.
func cliSaveBundle(t *testing.T, root string, input []byte) string {
	t.Helper()
	path := filepath.Join(root, "PRIVATE_PATH")
	cliWrite(t, path, input, 0o400)
	got := invokeCLI(t, cliEnvironment(root), nil, "import", "--json", "--software-release", "2.0.2", "--profile-index", "2", path)
	checkCLIStatus(t, got, 0, true)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(input))
}

type cliStoredIndex struct {
	Sources  []json.RawMessage `json:"sources"`
	Selected json.RawMessage   `json:"selected"`
}

func cliIndex(t *testing.T, app string) cliStoredIndex {
	t.Helper()
	var index cliStoredIndex
	if err := json.Unmarshal(cliRead(t, filepath.Join(app, "cache/source-index.json")), &index); err != nil {
		t.Fatal(err)
	}
	return index
}

func TestCLIPersistedSelectionRestart(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, input, result string }{
		{"duplicate_ids", string(cliRead(t, "../../internal/profileadapter/testdata/bundle.input.json")), cliPersistedResult},
		{"unsafe_and_missing_ids", `{"profiles":[{"id":"../PRIVATE_ID","name":"first","inputs":[]},{"id":"../PRIVATE_ID","name":null,"inputs":[]},{"inputs":[]}]}`, `{"root_kind":"bundle","export_version":{"present":false,"value":null},"profiles":[{"index":1,"name":"first","input_count":0},{"index":2,"name":null,"input_count":0},{"index":3,"name":null,"input_count":0}],"selected_profile_index":null,"warnings":[]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given: an explicit second ordinal and no remaining input file.
			root := t.TempDir()
			hash := cliSaveBundle(t, root, []byte(tc.input))
			app := filepath.Join(root, "XDG_DATA_HOME/azerlay")
			before := cliTree(t, root)
			// When: a separate process reads the committed selection.
			got := invokeCLI(t, cliEnvironment(root), nil, "profiles", "show", "--json")
			// Then: literal ordinal two survives, without ID-derived paths or writes.
			checkCLIStatus(t, got, 0, true)
			assertJSONEqual(t, got.stdout, cliSuccess("profiles show", cliSelected(tc.result, 2)))
			wantSelection := fmt.Sprintf(`{"source_hash":%q,"profile_index":2}`, hash)
			if !reflect.DeepEqual(parsedJSON(t, string(cliIndex(t, app).Selected)), parsedJSON(t, wantSelection)) {
				t.Fatalf("persisted selection differs from %s", wantSelection)
			}
			if !bytes.Equal(cliRead(t, filepath.Join(app, "sources", hash+".azeron")), []byte(tc.input)) {
				t.Fatal("stored original differs from exact input bytes")
			}
			count := 2
			if tc.name == "unsafe_and_missing_ids" {
				count = 3
			}
			wantPaths := []string{".", "cache", "cache/import.lock", "cache/source-index.json", "sources", "sources/" + hash + ".azeron", "profiles", "profiles/" + hash, "profiles/" + hash + "/bundle.json"}
			for n := 1; n <= count; n++ {
				wantPaths = append(wantPaths, fmt.Sprintf("profiles/%s/p%d.json", hash, n))
			}
			tree := cliTree(t, app)
			for _, path := range wantPaths {
				state, exists := tree[path]
				if !exists || state.Mode.IsDir() && state.Mode.Perm() != 0o700 || !state.Mode.IsDir() && state.Mode != 0o600 {
					t.Fatalf("missing or unsafe generated path %s: %+v", path, state)
				}
			}
			if len(tree) != len(wantPaths) || !reflect.DeepEqual(before, cliTree(t, root)) {
				t.Fatal("unexpected ID-derived paths or show changed the saved tree")
			}
		})
	}
}

func TestCLIDuplicateStorage(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"file", "stdin", "text", "byte_different"} {
		t.Run(route, func(t *testing.T) {
			// Given: first file provenance and a fixed historical timestamp.
			root := t.TempDir()
			input := cliRead(t, "../../internal/profileadapter/testdata/bundle.input.json")
			hash := cliSaveBundle(t, root, input)
			app := filepath.Join(root, "XDG_DATA_HOME/azerlay")
			indexPath := filepath.Join(app, "cache/source-index.json")
			var metadata struct {
				ImportedAt string `json:"imported_at"`
			}
			first := cliIndex(t, app)
			if len(first.Sources) != 1 {
				t.Fatalf("first import catalog entries=%d want=1", len(first.Sources))
			}
			if err := json.Unmarshal(first.Sources[0], &metadata); err != nil {
				t.Fatal(err)
			}
			cliWrite(t, indexPath, bytes.Replace(cliRead(t, indexPath), []byte(metadata.ImportedAt), []byte("2000-01-01T00:00:00Z"), 1), 0o600)
			first = cliIndex(t, app)
			wantMetadata := fmt.Sprintf(`{"source_hash":%q,"software_release":"2.0.2","source_scope":"azeron-software-export","input_kind":"reader","origin":{"kind":"file","path":%q},"imported_at":"2000-01-01T00:00:00Z","decoder_version":"1","normalizer_version":"5","model_schema_version":1,"profile_count":2,"export_version":"1e+09"}`, hash, filepath.Join(root, "PRIVATE_PATH"))
			if !reflect.DeepEqual(parsedJSON(t, string(first.Sources[0])), parsedJSON(t, wantMetadata)) {
				t.Fatalf("first metadata differs from known file attribution: %s", first.Sources[0])
			}
			before := cliTree(t, app)
			args := []string{"import", "--json", "--software-release", "2.0.2", "--profile-index", "1"}
			var stdin []byte
			switch route {
			case "file":
				path := filepath.Join(root, "second-file")
				cliWrite(t, path, input, 0o400)
				args = append(args, path)
			case "stdin":
				stdin, args = input, append(args, "-")
			case "text":
				args = append(args, "--text", string(input))
			case "byte_different":
				input = append(input, '\n')
				args = append(args, "--text", string(input))
			default:
				t.Fatal("unhandled input route")
			}
			// When: an admitted cross-route import explicitly changes selection.
			got := invokeCLI(t, cliEnvironment(root), stdin, args...)
			if route == "file" {
				if err := os.Remove(filepath.Join(root, "second-file")); err != nil {
					t.Fatal(err)
				}
			}
			// Then: exact-byte identity, first metadata and immutable artifacts survive.
			checkCLIStatus(t, got, 0, true)
			assertJSONEqual(t, got.stdout, cliSuccess("import", cliSelected(cliPersistedResult, 1)))
			after := cliTree(t, app)
			index := cliIndex(t, app)
			wantCount := 1
			selectedHash := hash
			if route == "byte_different" {
				wantCount = 2
				selectedHash = fmt.Sprintf("%x", sha256.Sum256(input))
				if !bytes.Equal(cliRead(t, filepath.Join(app, "sources", selectedHash+".azeron")), input) {
					t.Fatal("byte-different original was not retained separately")
				}
			} else if len(after) != len(before) {
				t.Fatal("identical bytes created duplicate storage artifacts")
			}
			if len(index.Sources) != wantCount || !bytes.Equal(index.Sources[0], first.Sources[0]) {
				t.Fatalf("dedup changed catalog count or first provenance: before=%s after=%s", first.Sources, index.Sources)
			}
			for path, state := range before {
				if path != "cache/source-index.json" && !reflect.DeepEqual(state, after[path]) {
					t.Fatalf("duplicate rewrote original/cache bytes or mode: %s", path)
				}
			}
			wantSelection := fmt.Sprintf(`{"source_hash":%q,"profile_index":1}`, selectedHash)
			if !reflect.DeepEqual(parsedJSON(t, string(index.Selected)), parsedJSON(t, wantSelection)) {
				t.Fatalf("duplicate did not commit changed selection: got=%s want=%s", index.Selected, wantSelection)
			}
			beforeShow := cliTree(t, root)
			show := invokeCLI(t, cliEnvironment(root), nil, "profiles", "show", "--json")
			checkCLIStatus(t, show, 0, true)
			assertJSONEqual(t, show.stdout, cliSuccess("profiles show", cliSelected(cliPersistedResult, 1)))
			if !reflect.DeepEqual(beforeShow, cliTree(t, root)) {
				t.Fatal("duplicate selection show wrote to HOME/XDG")
			}
		})
	}
}

func TestCLICacheRecoveryNoWrite(t *testing.T) {
	t.Parallel()
	for _, member := range []string{"bundle.json", "p2.json"} {
		for _, damage := range []string{"missing", "corrupt", "incompatible"} {
			t.Run(member+"/"+damage, func(t *testing.T) {
				// Given: committed original, deleted input, and one damaged derived member.
				root := t.TempDir()
				hash := cliSaveBundle(t, root, cliRead(t, "../../internal/profileadapter/testdata/bundle.input.json"))
				path := filepath.Join(root, "XDG_DATA_HOME/azerlay/profiles", hash, member)
				switch damage {
				case "missing":
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				case "corrupt":
					cliWrite(t, path, []byte("{"), 0o600)
				case "incompatible":
					cliWrite(t, path, bytes.Replace(cliRead(t, path), []byte(`"schema_version":1`), []byte(`"schema_version":999`), 1), 0o600)
				default:
					t.Fatal("unhandled cache damage")
				}
				before := cliTree(t, root)
				// When: a new process reconstructs from the saved original.
				got := invokeCLI(t, cliEnvironment(root), nil, "profiles", "show", "--json")
				// Then: the whole literal projection returns; damage is not repaired.
				checkCLIStatus(t, got, 0, true)
				assertJSONEqual(t, got.stdout, cliSuccess("profiles show", cliSelected(cliPersistedResult, 2)))
				if !reflect.DeepEqual(before, cliTree(t, root)) {
					t.Fatal("cache recovery wrote to HOME/XDG instead of remaining in memory")
				}
			})
		}
	}
}

func TestCLIStorageFailureNoAdoption(t *testing.T) {
	t.Parallel()
	for _, damage := range []string{"index", "original", "unsafe_mode", "blocked_store"} {
		for _, command := range []string{"show", "import"} {
			t.Run(damage+"/"+command, func(t *testing.T) {
				// Given: a committed second selection or an obstructed fresh store.
				root := t.TempDir()
				app := filepath.Join(root, "XDG_DATA_HOME/azerlay")
				input := cliRead(t, "../../internal/profileadapter/testdata/bundle.input.json")
				if damage == "blocked_store" {
					if err := os.Mkdir(filepath.Dir(app), 0o700); err != nil {
						t.Fatal(err)
					}
					cliWrite(t, app, []byte("PRIVATE_FIELD"), 0o600)
				} else {
					hash := cliSaveBundle(t, root, input)
					switch damage {
					case "index":
						cliWrite(t, filepath.Join(app, "cache/source-index.json"), []byte(`{"PRIVATE_FIELD":`), 0o600)
					case "original":
						cliWrite(t, filepath.Join(app, "sources", hash+".azeron"), []byte("PRIVATE_FIELD"), 0o600)
					case "unsafe_mode":
						if err := os.Chmod(app, 0o755); err != nil {
							t.Fatal(err)
						}
					default:
						t.Fatal("unhandled store damage")
					}
				}
				args := []string{"profiles", "show", "--json"}
				operation := "profiles show"
				if command == "import" {
					operation = "import"
					args = []string{"import", "--json", "--software-release", "2.0.2", "--profile-index", "1", "--text", string(input)}
				}
				before := cliTree(t, root)
				// When: reading or attempting changed selection through refused storage.
				got := invokeCLI(t, cliEnvironment(root), nil, args...)
				// Then: no success DTO, disclosure, repair, catalog reset or adoption.
				checkCLIFailure(t, got, 1, operation, "ERR_PROFILE_STORAGE", "storage", "null")
				checkCLIPrivate(t, got)
				if !reflect.DeepEqual(before, cliTree(t, root)) {
					t.Fatal("storage failure changed prior catalog/selection or adopted artifacts")
				}
			})
		}
	}
}

func TestCLIStoredDataPrivacy(t *testing.T) {
	t.Parallel()
	for _, jsonMode := range []bool{true, false} {
		t.Run(fmt.Sprintf("json=%t", jsonMode), func(t *testing.T) {
			// Given: private fields persisted through a private original file path.
			root := t.TempDir()
			env := cliEnvironment(root)
			path := filepath.Join(root, "PRIVATE_PATH")
			cliWrite(t, path, cliRead(t, "testdata/cli/private-fields.json"), 0o400)
			checkCLIStatus(t, invokeCLI(t, env, nil, "import", "--json", "--software-release", "2.0.2", path), 0, true)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			before := cliTree(t, root)
			args := []string{"profiles", "show"}
			if jsonMode {
				args = append(args, "--json")
			}
			// When: a new process projects stored private data to either public surface.
			got := invokeCLI(t, env, nil, args...)
			// Then: every permitted machine field has its literal value; no private DTO.
			checkCLIStatus(t, got, 0, true)
			if jsonMode {
				assertJSONEqual(t, got.stdout, cliSuccess("profiles show", cliSelected(cliPrivateResult, 1)))
			} else {
				checkCLIHuman(t, got.stdout, cliSelected(cliPrivateResult, 1))
			}
			checkCLIPrivate(t, got)
			if !reflect.DeepEqual(before, cliTree(t, root)) {
				t.Fatal("private stored-data display wrote to HOME/XDG")
			}
		})
	}
}

func checkCLIPrivate(t *testing.T, got cliOutput) {
	t.Helper()
	for _, sentinel := range []string{"PRIVATE_LABEL", "PRIVATE_MACRO", "PRIVATE_FIELD", "PRIVATE_ID", "PRIVATE_PATH", "PRIVATE_ARG"} {
		if strings.Contains(got.stdout, sentinel) || strings.Contains(got.stderr, sentinel) {
			t.Fatalf("private sentinel %s leaked: stdout=%q stderr=%q", sentinel, got.stdout, got.stderr)
		}
	}
	if strings.ContainsRune(got.stdout+got.stderr, '\x1b') {
		t.Fatal("unescaped terminal control in output")
	}
}

func TestCLIPrivateData(t *testing.T) {
	t.Parallel()
	env := cliEnvironment(t.TempDir())
	for _, command := range []string{"validate", "import"} {
		for _, jsonMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%t", command, jsonMode), func(t *testing.T) {
				args := []string{command, "--software-release", "2.0.2", "testdata/cli/private-fields.json"}
				if jsonMode {
					args = append(args, "--json")
				}
				got := invokeCLI(t, env, nil, args...)
				checkCLIPrivate(t, got)
				checkCLIStatus(t, got, 0, jsonMode)
				if got.stderr != "" {
					t.Fatalf("success stderr=%q", got.stderr)
				}
				result := cliPrivateResult
				if command == "import" {
					result = cliSelected(result, 1)
				}
				if jsonMode {
					assertJSONEqual(t, got.stdout, cliSuccess(command, result))
				} else {
					checkCLIHuman(t, got.stdout, result)
				}
			})
		}
	}
	for _, tc := range []struct {
		name, code, stage string
		status            int
		args              []string
	}{
		{"malformed", "ERR_IMPORT_JSON", "decode", 1, []string{"--text", `{"PRIVATE_FIELD":`}},
		{"missing_private_path", "ERR_IMPORT_ENCODING", "decode", 1, []string{filepath.Join(t.TempDir(), "PRIVATE_PATH")}},
		{"raw", "ERR_IMPORT_ROOT", "raw", 1, []string{"--text", `{"id":"PRIVATE_ID","inputs":false}`}},
		{"admission", "ERR_IMPORT_UNSUPPORTED_VERSION", "normalize", 1, []string{"testdata/cli/private-fields.json"}},
		{"usage", "ERR_CLI_USAGE", "usage", 2, []string{"--PRIVATE_ARG", "testdata/cli/private-fields.json"}},
	} {
		for _, jsonMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%t", tc.name, jsonMode), func(t *testing.T) {
				args := append([]string{"import"}, tc.args...)
				if jsonMode {
					args = append(args, "--json")
				}
				got := invokeCLI(t, env, nil, args...)
				checkCLIPrivate(t, got)
				checkCLIStatus(t, got, tc.status, jsonMode)
				if jsonMode {
					checkCLIFailure(t, got, tc.status, "import", tc.code, tc.stage, "null")
				} else if got.stdout != "" || !strings.HasPrefix(got.stderr, tc.code+" ("+tc.stage+"):") {
					t.Fatalf("unsafe/incorrect early failure streams: %+v", got)
				}
			})
		}
	}
}

// Direct bytes, entry sets and modes, including directories; no hashes and no
// audit files written into the environment being observed.
type cliFileState struct {
	Mode fs.FileMode
	Data []byte
}

func cliTree(t *testing.T, root string) map[string]cliFileState {
	t.Helper()
	state := make(map[string]cliFileState)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		item := cliFileState{Mode: info.Mode()}
		if !entry.IsDir() {
			item.Data, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		}
		state[rel] = item
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestCLINoPersistence(t *testing.T) {
	t.Parallel()
	for _, populated := range []bool{false, true} {
		for _, tc := range []struct {
			name, command, input string
			status               int
			options              []string
		}{
			{"validate", "validate", cliSingle, 0, nil},
			{"import", "import", cliSingle, 0, nil},
			{"explicit_selection", "import", cliBundle, 0, []string{"--profile-index", "2"}},
			{"ambiguous", "import", cliBundle, 1, nil},
			{"empty", "import", `{"profiles":[]}`, 1, nil},
			{"out_of_range", "import", cliBundle, 1, []string{"--profile-index", "4"}},
			{"decode", "import", "{", 1, nil},
			{"corrupt", "validate", string(cliRead(t, "testdata/cli/corrupt-lzma.txt")), 1, nil},
			{"raw", "import", `{"id":"synthetic","inputs":false}`, 1, nil},
			{"normalize", "import", cliMacroBundle(1001, false), 1, []string{"--profile-index", "1"}},
			{"usage", "import", cliSingle, 2, []string{"--profile-index", "0"}},
		} {
			t.Run(fmt.Sprintf("populated=%t/%s", populated, tc.name), func(t *testing.T) {
				root := t.TempDir()
				env := cliEnvironment(root)
				if populated {
					for _, entry := range env {
						key, path, _ := strings.Cut(entry, "=")
						if key != "HOME" && !strings.HasPrefix(key, "XDG_") {
							continue
						}
						if err := os.Mkdir(path, 0o750); err != nil {
							t.Fatal(err)
						}
						cliWrite(t, filepath.Join(path, "dirty-user-state"), []byte("PRIVATE_FIELD\x00\xff\nunsaved edits"), 0o640)
					}
				}
				source := filepath.Join(root, "readonly-export")
				cliWrite(t, source, []byte(tc.input), 0o400)
				before := cliTree(t, root)
				for _, jsonMode := range []bool{false, true} {
					args := append([]string{tc.command, "--software-release", "2.0.2", source}, tc.options...)
					if jsonMode {
						args = append(args, "--json")
					}
					got := invokeCLI(t, env, nil, args...)
					after := cliTree(t, root)
					if tc.command == "import" && tc.status == 0 {
						// Only successful imports may add app-owned data. Preserve the
						// source, existing ancestors and every unrelated HOME/XDG entry.
						for path := range after {
							if path == "XDG_DATA_HOME/azerlay" || strings.HasPrefix(path, "XDG_DATA_HOME/azerlay/") {
								delete(after, path)
							}
						}
						if !populated {
							delete(after, "XDG_DATA_HOME")
						}
					}
					if !reflect.DeepEqual(before, after) {
						t.Fatalf("CLI changed source/HOME/XDG bytes, entries or modes (json=%t): before=%v after=%v", jsonMode, before, after)
					}
					checkCLIStatus(t, got, tc.status, jsonMode)
					checkCLIPrivate(t, got)
				}
			})
		}
	}
}

func cliMacroBundle(count int, unknownLast bool) string {
	const step = `{"type":"Button","direction":"Full","duration":25,"keyCode":"KeyQ"}`
	steps := strings.Repeat(step+",", count-1) + step
	if unknownLast {
		steps += `,{"type":"Other","direction":"Full","duration":1,"keyCode":"PRIVATE_MACRO"}`
	}
	return `{"profiles":[{"id":"first","inputs":[]},{"id":"later","inputs":[{"types":["16","11","11"],"macro":{"v":1,"repeat":false,"steps":[` + steps + `]}}]}]}`
}

func TestCLIWholeExportFailure(t *testing.T) {
	t.Parallel()
	env := cliEnvironment(t.TempDir())
	const result = `{"root_kind":"bundle","export_version":{"present":false,"value":null},"profiles":[{"index":1,"name":null,"input_count":0},{"index":2,"name":null,"input_count":1}],"selected_profile_index":null,"warnings":[{"code":"WARN_IMPORT_UNKNOWN_BINDINGS","profile_index":2,"count":3}]}`
	for _, tc := range []struct {
		name    string
		count   int
		unknown bool
		status  int
	}{
		{"recognized_1000", 1000, false, 0},
		{"recognized_1001", 1001, false, 1},
		{"unknown_grammar_after_1001", 1001, true, 0},
	} {
		for _, command := range []string{"validate", "import"} {
			t.Run(tc.name+"/"+command, func(t *testing.T) {
				args := []string{command, "--json", "--software-release", "2.0.2", "-"}
				want := result
				if command == "import" {
					args = append(args, "--profile-index", "1")
					want = cliSelected(want, 1)
				}
				got := invokeCLI(t, env, []byte(cliMacroBundle(tc.count, tc.unknown)), args...)
				if tc.status == 1 {
					checkCLIFailure(t, got, 1, command, "ERR_IMPORT_LIMIT_EXCEEDED", "normalize", "null")
				} else {
					checkCLIStatus(t, got, 0, true)
					assertJSONEqual(t, got.stdout, cliSuccess(command, want))
				}
				checkCLIPrivate(t, got)
			})
		}
	}
}

// Extract data from the human surface. Ignore explanatory sentences, but
// retain all indexed rows and warnings so duplicates/extra rows cannot hide.
func checkCLIHuman(t *testing.T, text, expected string) {
	t.Helper()
	var want struct {
		RootKind string `json:"root_kind"`
		Version  struct {
			Present bool            `json:"present"`
			Value   json.RawMessage `json:"value"`
		} `json:"export_version"`
		Profiles []struct {
			Index  int     `json:"index"`
			Name   *string `json:"name"`
			Inputs int     `json:"input_count"`
		} `json:"profiles"`
		Selected *int `json:"selected_profile_index"`
		Warnings []struct {
			Code  string `json:"code"`
			Index int    `json:"profile_index"`
			Count int    `json:"count"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		t.Fatal(err)
	}
	wanted := map[string][]string{"Root kind": {want.RootKind}, "Profiles": {strconv.Itoa(len(want.Profiles))}}
	version := "<missing>"
	if want.Version.Present {
		version = string(want.Version.Value)
		var value string
		if version != "null" && json.Unmarshal(want.Version.Value, &value) == nil {
			version = strconv.Quote(value)
		}
	}
	wanted["Export version"] = []string{version}
	if want.Selected != nil {
		wanted["Selected profile"] = []string{strconv.Itoa(*want.Selected)}
	}
	for _, row := range want.Profiles {
		name := "<unnamed>"
		if row.Name != nil {
			name = strconv.Quote(*row.Name)
		}
		wanted["rows"] = append(wanted["rows"], fmt.Sprintf("%d|%s|%d", row.Index, name, row.Inputs))
	}
	for _, warning := range want.Warnings {
		wanted["warnings"] = append(wanted["warnings"], fmt.Sprintf("%s|%d|%d", warning.Code, warning.Index, warning.Count))
	}
	actual := make(map[string][]string)
	rowPattern := regexp.MustCompile(`^  ([0-9]+)\. (.+) \(([0-9]+) inputs\)$`)
	warningPattern := regexp.MustCompile(`^(WARN_[A-Z_]+): profile ([0-9]+), ([0-9]+) Unknown outcomes$`)
	for _, line := range strings.Split(text, "\n") {
		if row := rowPattern.FindStringSubmatch(line); row != nil {
			actual["rows"] = append(actual["rows"], strings.Join(row[1:], "|"))
		} else if warning := warningPattern.FindStringSubmatch(line); warning != nil {
			actual["warnings"] = append(actual["warnings"], strings.Join(warning[1:], "|"))
		} else if key, value, ok := strings.Cut(line, ": "); ok {
			actual[key] = append(actual[key], value)
		}
	}
	if !reflect.DeepEqual(actual, wanted) {
		t.Fatalf("human report fields=%v want=%v; output=%q", actual, wanted, text)
	}
}

func TestCLIReportSnapshots(t *testing.T) {
	t.Parallel()
	env := cliEnvironment(t.TempDir())
	for _, tc := range []struct {
		name, command, input, result, code string
		index                              string
		status                             int
	}{
		{"single", "validate", cliSingle, cliSingleResult, "", "", 0},
		{"implicit", "import", cliSingle, cliSelected(cliSingleResult, 1), "", "", 0},
		{"duplicates_missing_empty", "validate", cliBundle, cliBundleResult, "", "", 0},
		{"explicit_first", "import", cliBundle, cliSelected(cliBundleResult, 1), "", "1", 0},
		{"explicit_second", "import", cliBundle, cliSelected(cliBundleResult, 2), "", "02", 0},
		{"ambiguous", "import", cliBundle, cliBundleResult, "ERR_PROFILE_SELECTION_REQUIRED", "", 1},
		{"out_of_range", "import", cliBundle, cliBundleResult, "ERR_PROFILE_NOT_FOUND", "4", 1},
		{"empty_validate", "validate", `{"profiles":[]}`, cliEmptyResult, "", "", 0},
		{"empty_import", "import", `{"profiles":[]}`, cliEmptyResult, "ERR_PROFILE_NOT_FOUND", "", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{tc.command, "--software-release", "2.0.2", "--text", tc.input}
			if tc.index != "" {
				args = append(args, "--profile-index", tc.index)
			}
			got := invokeCLI(t, env, nil, append(args, "--json")...)
			if tc.status == 0 {
				checkCLIStatus(t, got, 0, true)
				assertJSONEqual(t, got.stdout, cliSuccess(tc.command, tc.result))
			} else {
				checkCLIFailure(t, got, 1, tc.command, tc.code, "selection", tc.result)
			}
			human := invokeCLI(t, env, nil, args...)
			checkCLIStatus(t, human, tc.status, false)
			checkCLIHuman(t, human.stdout, tc.result)
			if tc.status == 0 && human.stderr != "" || tc.status != 0 && !strings.HasPrefix(human.stderr, tc.code+" (selection):") {
				t.Fatalf("human stderr=%q", human.stderr)
			}
		})
	}
	for _, token := range []string{"null", "true", "false", "-0", "2.00e+30", "999999999999999999999999999", `"2.0.3"`, `""`} {
		for _, root := range []string{"single", "bundle"} {
			t.Run(root+"/version="+token, func(t *testing.T) {
				input := `{"id":"synthetic","inputs":[],"version":` + token + `}`
				result := cliSingleResult
				if root == "bundle" {
					input, result = `{"profiles":[],"version":`+token+`}`, cliEmptyResult
				}
				result = strings.Replace(result, `"present":false,"value":null`, `"present":true,"value":`+token, 1)
				args := []string{"validate", "--software-release", "2.0.2", "--text", input}
				got := invokeCLI(t, env, nil, append(args, "--json")...)
				checkCLIStatus(t, got, 0, true)
				assertJSONEqual(t, got.stdout, cliSuccess("validate", result))
				human := invokeCLI(t, env, nil, args...)
				checkCLIStatus(t, human, 0, true)
				checkCLIHuman(t, human.stdout, result)
			})
		}
	}
	t.Run("mapped_vs_near_match", func(t *testing.T) {
		mapped := string(cliRead(t, "../../internal/profileadapter/testdata/single.input.json"))
		near := strings.Replace(mapped, `"KeyU"`, `"KeyQ"`, 1)
		input := `{"profiles":[` + mapped + `,` + near + `]}`
		result := `{"root_kind":"bundle","export_version":{"present":false,"value":null},"profiles":[{"index":1,"name":"","input_count":1},{"index":2,"name":"","input_count":1}],"selected_profile_index":null,"warnings":[{"code":"WARN_IMPORT_UNKNOWN_BINDINGS","profile_index":1,"count":2},{"code":"WARN_IMPORT_UNKNOWN_BINDINGS","profile_index":2,"count":3}]}`
		args := []string{"validate", "--software-release", "2.0.2", "--text", input}
		got := invokeCLI(t, env, nil, append(args, "--json")...)
		checkCLIStatus(t, got, 0, true)
		assertJSONEqual(t, got.stdout, cliSuccess("validate", result))
		human := invokeCLI(t, env, nil, args...)
		checkCLIStatus(t, human, 0, true)
		checkCLIHuman(t, human.stdout, result)
	})
	t.Run("real_output_failure", func(t *testing.T) {
		// Linux /dev/full refuses the actual child's writes, without a mock
		// writer or a race against a pipe-closing goroutine.
		full, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := full.Close(); err != nil {
				t.Error(err)
			}
		})
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, cliBinary, "validate", "--json", "--software-release", "2.0.2", "--text", cliSingle)
		cmd.Env, cmd.Stdout = env, full
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err = cmd.Run()
		var exit *exec.ExitError
		if ctx.Err() != nil || !errors.As(err, &exit) || exit.ExitCode() != 1 || stderr.Len() != 0 {
			t.Fatalf("output failure err=%v context=%v stderr=%q", err, ctx.Err(), &stderr)
		}
	})
}
