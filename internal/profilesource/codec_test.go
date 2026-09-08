package profilesource

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
	"github.com/sh4869221b/azerlay/internal/profiledecode"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

// allow: SIZE_OK - The assigned scope keeps the schema-wide codec cases in this test file.
const fixtureHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func codecFixture(t *testing.T, name string) profile.ProfileBundle {
	t.Helper()
	input, err := os.ReadFile("../profileadapter/testdata/" + name + ".input.json")
	if err != nil {
		t.Fatal(err)
	}
	document, err := profiledecode.DecodeText(string(input))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := profileraw.Parse(document)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := profileadapter.Normalize(raw, profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export"})
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}
func codecRecord(bundle profile.ProfileBundle) diskSource {
	var version *string
	if bundle.Raw != nil && bundle.Raw.Version != nil {
		value := string(bundle.Raw.Version)
		version = &value
	}
	if bundle.RootKind == profile.RootSingle && bundle.Profiles[0].Raw.Version != nil {
		value := string(bundle.Profiles[0].Raw.Version)
		version = &value
	}
	return diskSource{SourceHash: fixtureHash, SoftwareRelease: bundle.Source.SoftwareRelease, SourceScope: bundle.Source.SourceScope, InputKind: "reader", Origin: diskOrigin{Kind: "stdin"}, ImportedAt: "2026-09-08T01:02:03.123456789Z", DecoderVersion: "1", NormalizerVersion: "1", ModelSchemaVersion: 1, ProfileCount: len(bundle.Profiles), ExportVersion: version}
}
func opaqueFixture(t *testing.T) profile.ProfileBundle {
	bundle := codecFixture(t, "bundle")
	bundle.Raw.Version = json.RawMessage(" \t1e+09\r\n")
	bundle.Raw.Unknown = map[string]json.RawMessage{"null": json.RawMessage("null"), "escape": json.RawMessage(`"\u0041\\\""`), "space": json.RawMessage(" { \"x\": [ -0, 1.00e+2 ] } "), "empty": json.RawMessage(`""`)}
	bundle.Profiles[0].Raw.ID = nil
	bundle.Profiles[0].Raw.Name = json.RawMessage("null")
	bundle.Profiles[0].Raw.Unknown = map[string]json.RawMessage{}
	bundle.Profiles[1].Raw.Unknown = nil
	empty := ""
	bundle.Profiles[0].Name = &empty
	bundle.Profiles[0].Controls[0].Bindings[2].Actions = []profile.Action{
		{Kind: profile.ActionKeyboard, Code: profile.KEY_I, Modifiers: []profile.CanonicalCode{}},
		{Kind: profile.ActionKeyboard, Code: profile.KEY_P},
		{Kind: profile.ActionKeyboard, Code: profile.KEY_P},
	}
	return bundle
}
func TestCacheRoundTrip(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"bundle", "single", "opaque", "absent"} {
		t.Run(name, func(t *testing.T) {
			// Given: real adapter output, plus opaque carrier edge cases without semantic interpretation.
			bundle := codecFixture(t, "bundle")
			switch name {
			case "single":
				bundle = codecFixture(t, "single")
			case "opaque":
				bundle = opaqueFixture(t)
			case "absent":
				bundle.Raw.Version = nil
				bundle.Profiles[0] = profile.Profile{Controls: []profile.ControlBinding{}}
			}
			source := codecRecord(bundle)
			index := diskIndex{SchemaVersion: 1, Sources: []diskSource{source}, Selected: &diskSelection{SourceHash: fixtureHash, ProfileIndex: len(bundle.Profiles)}}
			// When: serialize and reconstruct the complete cache set and index.
			data, err := encodeIndex(index)
			if err != nil {
				t.Fatal(err)
			}
			gotIndex, err := decodeIndex(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			data, err = encodeBundleCache(source, bundle)
			if err != nil {
				t.Fatal(err)
			}
			readers := make([]io.Reader, len(bundle.Profiles))
			for i := range bundle.Profiles {
				member, e := encodeProfileCache(source, bundle, i+1)
				if e != nil {
					t.Fatal(e)
				}
				readers[i] = bytes.NewReader(member)
			}
			got, err := decodeCacheSet(source, bytes.NewReader(data), readers)
			if err != nil {
				t.Fatal(err)
			}
			// Then: all model fields, nil/empty carriers and duplicate-ID order equal the input.
			if !reflect.DeepEqual(got, bundle) {
				t.Fatalf("model changed: got %#v; want %#v", got, bundle)
			}
			if !reflect.DeepEqual(gotIndex, index) {
				t.Fatalf("index changed: %#v", gotIndex)
			}
		})
	}
}
func TestStorageCodecRejectsInvalid(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"unsupported schema":       `{"schema_version":999,"sources":[],"selected":null}`,
		"missing selected":         `{"schema_version":1,"sources":[]}`,
		"null sources":             `{"schema_version":1,"sources":null,"selected":null}`,
		"trailing value":           `{"schema_version":1,"sources":[],"selected":null} {}`,
		"duplicate decoded key":    `{"schema_version":1,"sources":[],"selected":null,"\u0073elected":null}`,
		"unknown nested duplicate": `{"schema_version":1,"sources":[],"selected":null,"future":{"a":1,"\u0061":2}}`,
		"missing source":           `{"schema_version":1,"sources":[],"selected":{"source_hash":"` + fixtureHash + `","profile_index":1}}`,
		"bad hash":                 `{"schema_version":1,"sources":[],"selected":{"source_hash":"../unsafe","profile_index":1}}`,
		"bad ordinal":              `{"schema_version":1,"sources":[],"selected":{"source_hash":"` + fixtureHash + `","profile_index":0}}`,
	} {
		t.Run(name, func(t *testing.T) {
			// Given: a malformed committed index.
			// When
			got, err := decodeIndex(strings.NewReader(input))
			// Then: a typed codec failure and zero result, never a partial catalog.
			var failure *codecError
			if !errors.As(err, &failure) || !reflect.DeepEqual(got, diskIndex{}) {
				t.Fatalf("got %#v, %v; want zero index and codecError", got, err)
			}
		})
	}
	// Given: valid real-adapter artifacts, with one independent boundary fault per case.
	bundle := codecFixture(t, "bundle")
	source := codecRecord(bundle)
	indexData, err := encodeIndex(diskIndex{SchemaVersion: 1, Sources: []diskSource{source}, Selected: &diskSelection{SourceHash: fixtureHash, ProfileIndex: 2}})
	if err != nil {
		t.Fatal(err)
	}
	bundleData, err := encodeBundleCache(source, bundle)
	if err != nil {
		t.Fatal(err)
	}
	profileData, err := encodeProfileCache(source, bundle, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []storageMutation{
		{"sources/0/source_hash", `"ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`},
		{"sources/0/software_release", `null`}, {"sources/0/source_scope", `""`},
		{"sources/0/input_kind", `"file"`}, {"sources/0/origin/kind", `"unknown"`}, {"sources/0/origin/path", `false`},
		{"sources/0/origin", `{"kind":"file","path":42}`},
		{"sources/0/imported_at", `"2026-09-08T01:02:03+01:00"`}, {"sources/0/imported_at", `"not-time"`},
		{"sources/0/decoder_version", `""`}, {"sources/0/normalizer_version", `null`}, {"sources/0/model_schema_version", `0`},
		{"sources/0/profile_count", `0`}, {"sources/0/profile_count", `513`}, {"sources/0/export_version", `1`},
		{"selected/profile_index", `3`}, {"selected/profile_index", `null`},
		{"selected/source_hash", `"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"`},
	} {
		t.Run("index/"+change.path+"/"+change.value, func(t *testing.T) {
			// When
			got, err := decodeIndex(bytes.NewReader(mutateStorage(t, indexData, change)))
			// Then
			var failure *codecError
			if !errors.As(err, &failure) || !reflect.DeepEqual(got, diskIndex{}) {
				t.Fatalf("accepted invalid index: %#v, %v", got, err)
			}
		})
	}
	for _, change := range []storageMutation{
		{"schema_version", `999`}, {"source_hash", `"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"`},
		{"decoder_version", `"2"`}, {"normalizer_version", `"2"`}, {"model_schema_version", `2`},
		{"bundle/schema_version", `2`}, {"bundle/source/software_release", `"9.9.9"`}, {"bundle/source/source_scope", `"other"`},
		{"bundle/root_kind", `"single"`}, {"bundle/root_kind", `"other"`}, {"bundle/raw", `null`}, {"bundle/profiles", `[]`},
		{"bundle/raw/version", `null`}, {"bundle/raw/unknown", `{"bad":null}`},
		{"bundle/profiles/0/controls", `null`}, {"bundle/profiles/0/raw/id", `1`},
		{"bundle/profiles/0/controls/0/raw/profile_index", `1`}, {"bundle/profiles/0/controls/0/raw/profile_index", `null`},
		{"bundle/profiles/0/controls/0/raw/input_index", `1`}, {"bundle/profiles/0/controls/0/raw/root_kind", `"single"`},
		{"bundle/profiles/0/controls/0/raw/fields", `{"opaque":null}`},
		{"bundle/profiles/0/controls/0/bindings", `[]`},
		{"bundle/profiles/0/controls/0/bindings/0/trigger", `"long"`},
		{"bundle/profiles/0/controls/0/bindings/0/kind", `"alien"`},
		{"bundle/profiles/0/controls/0/bindings/0/unknown", `null`},
		{"bundle/profiles/0/controls/0/bindings/0/actions", `[]`},
		{"bundle/profiles/0/controls/0/bindings/0/trigger_delay_ms", `1`},
		{"bundle/profiles/0/controls/0/bindings/0/unknown/reason", `null`},
		{"bundle/profiles/0/controls/0/bindings/1/unknown", `{"reason":"unmapped_binding"}`},
		{"bundle/profiles/0/controls/0/bindings/1/actions", `null`},
		{"bundle/profiles/0/controls/0/bindings/1/trigger_delay_ms", `null`},
		{"bundle/profiles/0/controls/0/bindings/1/trigger_delay_ms", `-1`},
		{"bundle/profiles/0/controls/0/bindings/1/trigger_interval_ms", `1`},
		{"bundle/profiles/0/controls/0/bindings/1/release_behavior", `"toggle"`},
		{"bundle/profiles/0/controls/0/bindings/1/actions/0/kind", `"mouse"`},
		{"bundle/profiles/0/controls/0/bindings/1/actions/0/code", `"KEY_X"`},
		{"bundle/profiles/0/controls/0/bindings/1/actions/0/modifiers", `[null]`},
		{"bundle/profiles/0/controls/0/bindings/2/release_behavior", `"regular"`},
	} {
		t.Run("bundle/"+change.path+"/"+change.value, func(t *testing.T) {
			// When
			got, err := decodeBundleCache(bytes.NewReader(mutateStorage(t, bundleData, change)), source)
			// Then
			var failure *codecError
			if !errors.As(err, &failure) || !reflect.DeepEqual(got, profile.ProfileBundle{}) {
				t.Fatalf("accepted invalid bundle: %#v, %v", got, err)
			}
		})
	}
	for _, change := range []storageMutation{
		{"schema_version", `999`}, {"source_hash", `"../bad"`}, {"decoder_version", `"2"`}, {"normalizer_version", `"2"`}, {"model_schema_version", `2`},
		{"profile_index", `0`}, {"profile_index", `2`}, {"profile_index", `3`}, {"profile", `null`},
		{"profile/controls/0/raw/profile_index", `1`},
	} {
		t.Run("profile/"+change.path, func(t *testing.T) {
			// When
			got, err := decodeProfileCache(bytes.NewReader(mutateStorage(t, profileData, change)), source, 1)
			// Then
			var failure *codecError
			if !errors.As(err, &failure) || !reflect.DeepEqual(got, profile.Profile{}) {
				t.Fatalf("accepted invalid profile: %#v, %v", got, err)
			}
		})
	}
	for _, artifact := range []struct {
		name   string
		data   []byte
		decode func([]byte) error
	}{
		{"index", indexData, func(b []byte) error { _, err := decodeIndex(bytes.NewReader(b)); return err }},
		{"bundle", bundleData, func(b []byte) error { _, err := decodeBundleCache(bytes.NewReader(b), source); return err }},
		{"profile", profileData, func(b []byte) error { _, err := decodeProfileCache(bytes.NewReader(b), source, 1); return err }},
	} {
		// Every DTO field is mandatory, including nullable pointers and raw maps.
		for _, path := range storageFieldPaths(t, artifact.data) {
			t.Run(artifact.name+"/missing/"+path, func(t *testing.T) {
				// When
				err := artifact.decode(mutateStorage(t, artifact.data, storageMutation{path, ""}))
				// Then
				var failure *codecError
				if !errors.As(err, &failure) {
					t.Fatalf("missing field accepted: %v", err)
				}
			})
		}
		for name, suffix := range map[string]string{"trailing": " {}", "duplicate": `,"schema_version":1}`} {
			t.Run(artifact.name+"/"+name, func(t *testing.T) {
				input := append([]byte(nil), artifact.data...)
				switch name {
				case "trailing":
					input = append(input, suffix...)
				case "duplicate":
					input = append(input[:len(input)-1], (suffix + "}")...)
				}
				// When
				err := artifact.decode(input)
				// Then
				var failure *codecError
				if !errors.As(err, &failure) {
					t.Fatalf("malformed storage accepted: %v", err)
				}
			})
		}
	}
	t.Run("duplicate sources", func(t *testing.T) {
		// Given
		input, err := json.Marshal(diskIndex{SchemaVersion: 1, Sources: []diskSource{source, source}})
		if err != nil {
			t.Fatal(err)
		}
		// When
		got, err := decodeIndex(bytes.NewReader(input))
		// Then
		var failure *codecError
		if !errors.As(err, &failure) || !reflect.DeepEqual(got, diskIndex{}) {
			t.Fatalf("duplicate accepted: %#v, %v", got, err)
		}
	})
}

type storageMutation struct {
	path  string
	value string
}

func mutateStorage(t *testing.T, data []byte, change storageMutation) []byte {
	t.Helper()
	head, tail, nested := strings.Cut(change.path, "/")
	if len(data) > 0 && data[0] == '[' {
		var items []json.RawMessage
		if err := json.Unmarshal(data, &items); err != nil {
			t.Fatal(err)
		}
		index, err := strconv.Atoi(head)
		if err != nil || index < 0 || index >= len(items) || !nested {
			t.Fatalf("invalid array path: %s", change.path)
		}
		items[index] = mutateStorage(t, items[index], storageMutation{tail, change.value})
		result, err := json.Marshal(items)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	value, exists := fields[head]
	if !exists {
		t.Fatalf("missing mutation target: %s", change.path)
	}
	switch {
	case nested:
		fields[head] = mutateStorage(t, value, storageMutation{tail, change.value})
	case change.value == "":
		delete(fields, head)
	default:
		fields[head] = json.RawMessage(change.value)
	}
	result, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func storageFieldPaths(t *testing.T, data []byte) []string {
	t.Helper()
	var paths []string
	if len(data) == 0 {
		return paths
	}
	switch data[0] {
	case '{':
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			t.Fatal(err)
		}
		for key, value := range fields {
			paths = append(paths, key)
			if key == "fields" || key == "unknown" && (len(value) == 0 || !bytes.Contains(value, []byte(`"reason"`))) {
				continue
			}
			for _, child := range storageFieldPaths(t, value) {
				paths = append(paths, key+"/"+child)
			}
		}
	case '[':
		var items []json.RawMessage
		if err := json.Unmarshal(data, &items); err != nil {
			t.Fatal(err)
		}
		for i, value := range items {
			for _, child := range storageFieldPaths(t, value) {
				paths = append(paths, strconv.Itoa(i)+"/"+child)
			}
		}
	}
	return paths
}

func TestCacheSetRejectsStaleState(t *testing.T) {
	t.Parallel()
	// Given: both complete artifacts have the same duplicate IDs but different profiles.
	bundle := codecFixture(t, "bundle")
	source := codecRecord(bundle)
	data, err := encodeBundleCache(source, bundle)
	if err != nil {
		t.Fatal(err)
	}
	first, err := encodeProfileCache(source, bundle, 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := encodeProfileCache(source, bundle, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"missing", "swapped", "different profile", "different raw token", "stale revision", "invalid requested ordinal"} {
		t.Run(name, func(t *testing.T) {
			readers := []io.Reader{bytes.NewReader(first), bytes.NewReader(second)}
			switch name {
			case "missing":
				readers = readers[:1]
			case "swapped":
				readers = []io.Reader{bytes.NewReader(second), bytes.NewReader(first)}
			case "different profile":
				readers[1] = bytes.NewReader(mutateStorage(t, second, storageMutation{"profile/name", `"different"`}))
			case "different raw token":
				readers[1] = bytes.NewReader(mutateStorage(t, second, storageMutation{"profile/raw/id", `"null"`}))
			case "stale revision":
				readers[1] = bytes.NewReader(mutateStorage(t, second, storageMutation{"normalizer_version", `"2"`}))
			case "invalid requested ordinal":
				readers = append(readers, bytes.NewReader(second))
			}
			// When
			got, err := decodeCacheSet(source, bytes.NewReader(data), readers)
			// Then: no partial bundle can escape a bad member, even with matching Azeron IDs.
			var failure *codecError
			if !errors.As(err, &failure) || !reflect.DeepEqual(got, profile.ProfileBundle{}) {
				t.Fatalf("cache set accepted: %#v, %v", got, err)
			}
		})
	}
}

func TestIndexAcceptedMetadata(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"historical revisions", "nullable file path", "fraction spelling", "unknown key", "empty store", "absent export version"} {
		t.Run(name, func(t *testing.T) {
			// Given: admitted metadata shapes, including historical interpretation and optional provenance.
			bundle := codecFixture(t, "bundle")
			source := codecRecord(bundle)
			switch name {
			case "historical revisions":
				source.DecoderVersion = "historical"
				source.NormalizerVersion = "old"
				source.ModelSchemaVersion = 2
			case "nullable file path":
				source.Origin = diskOrigin{Kind: "file"}
			case "fraction spelling":
				source.ImportedAt = "2026-09-08T01:02:03.120000000Z"
			case "absent export version":
				source.ExportVersion = nil
			}
			other := source
			other.SourceHash = strings.Repeat("f", 64)
			other.ProfileCount = 1
			index := diskIndex{SchemaVersion: 1, Sources: []diskSource{source, other}, Selected: &diskSelection{SourceHash: fixtureHash, ProfileIndex: 2}}
			if name == "empty store" {
				index.Sources = []diskSource{}
				index.Selected = nil
			}
			// When
			data, err := encodeIndex(index)
			if err != nil {
				t.Fatal(err)
			}
			if name == "unknown key" {
				data = append(data[:len(data)-1], `,"SCHEMA_VERSION":999,"future":{"data":true}}`...)
			}
			got, err := decodeIndex(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			// Then: source order, exact metadata and selection are retained; aliases cannot override known keys.
			if !reflect.DeepEqual(got, index) {
				t.Fatalf("index changed: %#v", got)
			}
		})
	}
}

func TestStorageBounds(t *testing.T) {
	t.Parallel()
	t.Run("index read exact and overflow", func(t *testing.T) {
		// Given: a valid document padded to the real index cap.
		prefix := `{"schema_version":1,"sources":[],"selected":null}`
		for _, extra := range []int{0, 1} {
			t.Run(strconv.Itoa(extra), func(t *testing.T) {
				reader := strings.NewReader(prefix + strings.Repeat(" ", maxIndexBytes-len(prefix)+extra))
				// When
				got, err := decodeIndex(reader)
				// Then
				if extra == 0 {
					if err != nil || got.Sources == nil {
						t.Fatalf("exact cap: %#v, %v", got, err)
					}
				} else if !errors.Is(err, errCodecLimit) || !reflect.DeepEqual(got, diskIndex{}) {
					t.Fatalf("overflow: %#v, %v", got, err)
				}
			})
		}
	})
	t.Run("candidate index overflow", func(t *testing.T) {
		// Given
		source := codecRecord(codecFixture(t, "bundle"))
		path := strings.Repeat("x", maxIndexBytes)
		source.Origin = diskOrigin{Kind: "file", Path: &path}
		// When
		data, err := encodeIndex(diskIndex{SchemaVersion: 1, Sources: []diskSource{source}})
		// Then
		if data != nil || !errors.Is(err, errCodecLimit) {
			t.Fatalf("oversize candidate: %d, %v", len(data), err)
		}
	})
	t.Run("shared cache boundary cap plus one", func(t *testing.T) {
		// Given: the exact cache read/write helpers with a small byte budget.
		const document = `{"schema_version":1,"sources":[],"selected":null}`
		input := strings.NewReader(document)
		var index diskIndex
		// When
		err := readStorage(input, &index, 8)
		// Then: bounded reading consumes only cap+1, and the disk caps remain the specified constants.
		if !errors.Is(err, errCodecLimit) || input.Len() != len(document)-9 || maxIndexBytes != 16<<20 || maxCacheBytes != 512<<20 {
			t.Fatalf("bounded read: remaining %d, %v", input.Len(), err)
		}
	})
	t.Run("cache encoding cap", func(t *testing.T) {
		// Given
		value := diskBundleEnvelope{}
		// When
		data, err := marshalStorage(value, 8)
		// Then
		if data != nil || !errors.Is(err, errCodecLimit) {
			t.Fatalf("oversize encoding: %d, %v", len(data), err)
		}
	})
	t.Run("reader cause", func(t *testing.T) {
		// Given: bytes that look valid followed by a terminal reader error.
		reader := io.MultiReader(strings.NewReader(`{"schema_version":1,"sources":[],"selected":null}`), codecErrorReader{})
		// When
		got, err := decodeIndex(reader)
		// Then
		if !errors.Is(err, io.ErrUnexpectedEOF) || !reflect.DeepEqual(got, diskIndex{}) {
			t.Fatalf("read error lost: %#v, %v", got, err)
		}
	})
}

type codecErrorReader struct{}

func (codecErrorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
