package profilesource

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
	"github.com/sh4869221b/azerlay/internal/profiledecode"
	"github.com/sh4869221b/azerlay/internal/profileraw"
	"github.com/ulikunitz/xz/lzma"
)

func TestPrepareExactOriginal(t *testing.T) {
	// Given: synthetic exports with ordered duplicate IDs, bindings and opaque tokens.
	fixture, err := os.ReadFile("../profileadapter/testdata/bundle.input.json")
	if err != nil {
		t.Fatal(err)
	}
	const single = `{"id":null,"name":"","inputs":[]}`
	compressed := compressedExport(t, string(fixture))
	cases := []struct{ name, original, payload string }{
		{"bundle", string(fixture), string(fixture)},
		{"wrapped", " \n```\n'''" + string(fixture) + "'''\n```\t", string(fixture)},
		{"single", single, single},
		{"empty bundle", `{"profiles":[]}`, `{"profiles":[]}`},
		{"base64", base64.RawURLEncoding.EncodeToString(compressed), string(fixture)},
		{"raw lzma", string(compressed), string(fixture)},
		{"source cap", single + strings.Repeat(" ", (16<<20)-len(single)), single},
		{"macro cap", macroExport(1000), macroExport(1000)},
	}
	for _, tc := range cases {
		for _, kind := range []string{"text", "reader"} {
			if tc.name == "raw lzma" && kind == "text" {
				continue // Binary text rejection is covered by TestPrepareFailure.
			}
			t.Run(tc.name+"/"+kind, func(t *testing.T) {
				// Given: the expected full model comes from the original fixture, not Prepared.
				source := profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export"}
				document, err := profiledecode.DecodeText(tc.payload)
				if err != nil {
					t.Fatal(err)
				}
				raw, err := profileraw.Parse(document)
				if err != nil {
					t.Fatal(err)
				}
				want, err := profileadapter.Normalize(raw, source)
				if err != nil {
					t.Fatal(err)
				}
				input := []byte(tc.original)
				var got Prepared
				// When: prepare through the requested route, then change caller-owned inputs.
				switch kind {
				case "text":
					got, err = PrepareText(string(input), source)
				case "reader":
					got, err = PrepareReader(bytes.NewReader(input), source)
				default:
					t.Fatalf("unexpected route %q", kind)
				}
				clear(input)
				source.SoftwareRelease = "9.9.9"
				// Then: exact originals, capture route and every normalized field survive independently.
				if err != nil {
					t.Fatalf("prepare error = %v", err)
				}
				if string(got.original) != tc.original {
					t.Fatal("original bytes changed")
				}
				if got.inputKind != kind {
					t.Fatalf("input kind = %q, want %q", got.inputKind, kind)
				}
				if !reflect.DeepEqual(got.Bundle(), want) {
					t.Fatal("complete normalized bundle differs")
				}
				// The captured original and model also have independent storage.
				clear(got.original)
				if !reflect.DeepEqual(got.Bundle(), want) {
					t.Fatal("model aliases original capture")
				}
			})
		}
	}
}

func TestPrepareFailure(t *testing.T) {
	// Given: an isolated data home and failures at each existing boundary.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", home)
	source := profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export"}
	const valid = `{"id":null,"inputs":[]}`
	compressed := compressedExport(t, valid)
	cases := []struct {
		name, input string
		source      profile.SourceMetadata
		want        error
	}{
		{"malformed", `{`, source, &profiledecode.DecodeError{Code: profiledecode.ERR_IMPORT_JSON}},
		{"trailing JSON", valid + `{}`, source, &profiledecode.DecodeError{Code: profiledecode.ERR_IMPORT_JSON}},
		{"duplicate key", `{"id":null,"id":1,"inputs":[]}`, source, &profileraw.ParseError{Code: profileraw.ERR_IMPORT_ROOT}},
		{"invalid inputs", `{"id":null,"inputs":null}`, source, &profileraw.ParseError{Code: profileraw.ERR_IMPORT_ROOT}},
		{"container version", `{"version":{},"profiles":[]}`, source, &profileraw.ParseError{Code: profileraw.ERR_IMPORT_UNSUPPORTED_VERSION}},
		{"profile cap", `{"profiles":[` + strings.Repeat(`{"inputs":[]},`, 512) + `{"inputs":[]}]}`, source, &profileraw.ParseError{Code: profileraw.ERR_IMPORT_LIMIT_EXCEEDED}},
		{"source overflow", valid + strings.Repeat(" ", (16<<20)+1-len(valid)), source, &profiledecode.DecodeError{Code: profiledecode.ERR_IMPORT_LIMIT_EXCEEDED}},
		{"unsupported release", valid, profile.SourceMetadata{SoftwareRelease: "2.0.3", SourceScope: source.SourceScope}, &profileadapter.NormalizeError{Code: profileadapter.ERR_IMPORT_UNSUPPORTED_VERSION}},
		{"unsupported scope", valid, profile.SourceMetadata{SoftwareRelease: source.SoftwareRelease, SourceScope: "file"}, &profileadapter.NormalizeError{Code: profileadapter.ERR_IMPORT_UNSUPPORTED_VERSION}},
		{"later profile overflow", macroExport(1001), source, &profileadapter.NormalizeError{Code: profileadapter.ERR_IMPORT_LIMIT_EXCEEDED}},
		{"truncated compressed", base64.RawURLEncoding.EncodeToString(compressed[:len(compressed)-1]), source, &profiledecode.DecodeError{Code: profiledecode.ERR_IMPORT_LZMA_CORRUPT}},
	}
	for _, tc := range cases {
		for _, kind := range []string{"text", "reader"} {
			t.Run(tc.name+"/"+kind, func(t *testing.T) {
				// Given: an earlier success must not leak into the failed result.
				got, err := PrepareText(valid, source)
				if err != nil {
					t.Fatal(err)
				}
				// When
				switch kind {
				case "text":
					got, err = PrepareText(tc.input, tc.source)
				case "reader":
					got, err = PrepareReader(strings.NewReader(tc.input), tc.source)
				default:
					t.Fatalf("unexpected route %q", kind)
				}
				// Then
				assertPrepareError(t, err, tc.want)
				if !reflect.DeepEqual(got, Prepared{}) {
					t.Fatal("failure returned partial or stale Prepared")
				}
			})
		}
	}
	t.Run("binary text stays text", func(t *testing.T) {
		// Given / When
		got, err := PrepareText(string(compressed), source)
		// Then
		assertPrepareError(t, err, &profiledecode.DecodeError{Code: profiledecode.ERR_IMPORT_ENCODING})
		if !reflect.DeepEqual(got, Prepared{}) {
			t.Fatal("binary text returned a candidate")
		}
	})
	cause := errors.New("synthetic terminal reader failure")
	for _, sameRead := range []bool{false, true} {
		t.Run(map[bool]string{false: "error after data", true: "data with error"}[sameRead], func(t *testing.T) {
			// Given: even complete valid JSON must not hide a terminal I/O error.
			reader := &terminalReader{data: []byte(valid), cause: cause, sameRead: sameRead}
			// When
			got, err := PrepareReader(reader, source)
			// Then
			assertPrepareError(t, err, &profiledecode.DecodeError{Code: profiledecode.ERR_IMPORT_ENCODING})
			if !errors.Is(err, cause) {
				t.Fatal("reader cause lost")
			}
			if !reflect.DeepEqual(got, Prepared{}) {
				t.Fatal("terminal error returned a candidate")
			}
		})
	}
	t.Run("nil reader", func(t *testing.T) {
		// Given / When
		got, err := PrepareReader(nil, source)
		// Then
		assertPrepareError(t, err, &profiledecode.DecodeError{Code: profiledecode.ERR_IMPORT_ENCODING})
		if !reflect.DeepEqual(got, Prepared{}) {
			t.Fatal("nil reader returned a candidate")
		}
	})
	t.Run("bounded reader", func(t *testing.T) {
		// Given: extra data beyond the decoder's cap-plus-one read.
		reader := strings.NewReader(strings.Repeat(" ", (16<<20)+32))
		// When
		got, err := PrepareReader(reader, source)
		// Then
		assertPrepareError(t, err, &profiledecode.DecodeError{Code: profiledecode.ERR_IMPORT_LIMIT_EXCEEDED})
		if reader.Len() != 31 {
			t.Fatalf("unread bytes = %d, want 31", reader.Len())
		}
		if !reflect.DeepEqual(got, Prepared{}) {
			t.Fatal("oversize reader returned a candidate")
		}
	})
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("preparation wrote to HOME/data home")
	}
}

func assertPrepareError(t *testing.T, err, want error) {
	t.Helper()
	if err == nil {
		t.Fatal("error = nil, want typed failure")
	}
	matched := false
	switch want.(type) {
	case *profiledecode.DecodeError:
		var typed *profiledecode.DecodeError
		matched = errors.As(err, &typed)
	case *profileraw.ParseError:
		var typed *profileraw.ParseError
		matched = errors.As(err, &typed)
	case *profileadapter.NormalizeError:
		var typed *profileadapter.NormalizeError
		matched = errors.As(err, &typed)
	default:
		t.Fatalf("unexpected error type %T", want)
	}
	if !matched || err.Error() != want.Error() {
		t.Fatalf("error = %T %v, want %T %v", err, err, want, want)
	}
}

func compressedExport(t *testing.T, payload string) []byte {
	t.Helper()
	var compressed bytes.Buffer
	writer, err := (lzma.WriterConfig{DictCap: 4096}).NewWriter(&compressed)
	if err != nil {
		t.Fatal(err)
	}
	header := binary.BigEndian.AppendUint32([]byte{0xdb}, uint32(len(payload)))
	if _, err := writer.Write(append(header, payload...)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return compressed.Bytes()
}

func macroExport(count int) string {
	const step = `{"type":"Delay","direction":"Full","duration":1}`
	return `{"profiles":[{"name":"valid first","inputs":[]},{"inputs":[{"types":["16","11","11"],"macro":{"v":1,"repeat":false,"steps":[` + strings.Repeat(step+",", count-1) + step + `]}}]}]}`
}

type terminalReader struct {
	data     []byte
	cause    error
	sameRead bool
}

func (r *terminalReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, r.cause
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	if r.sameRead && len(r.data) == 0 {
		return n, r.cause
	}
	return n, nil
}

var _ io.Reader = (*terminalReader)(nil)
