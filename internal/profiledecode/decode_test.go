package profiledecode

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDecodeSupportedCorpus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		json string
		kind RootKind
	}{
		{name: "raw bundle", text: `{"profiles":[]}`, json: `{"profiles":[]}`, kind: RootBundle},
		{name: "raw bundle with optional version", text: `{"version":"synthetic","profiles":[]}`, json: `{"version":"synthetic","profiles":[]}`, kind: RootBundle},
		{name: "raw single by id", text: `{"id":"synthetic","inputs":[]}`, json: `{"id":"synthetic","inputs":[]}`, kind: RootSingle},
		{name: "raw single by name", text: `{"name":"synthetic","inputs":[]}`, json: `{"name":"synthetic","inputs":[]}`, kind: RootSingle},
		{name: "leading BOM and space", text: " \n\ufeff{\"profiles\":[]}\t", json: `{"profiles":[]}`, kind: RootBundle},
		{name: "fenced", text: "```\n{\"profiles\":[]}\n```", json: `{"profiles":[]}`, kind: RootBundle},
		{name: "single quoted", text: `' {"profiles":[]} '`, json: `{"profiles":[]}`, kind: RootBundle},
		{name: "double quoted", text: `" {"profiles":[]} "`, json: `{"profiles":[]}`, kind: RootBundle},
		{name: "triple single quoted", text: `''' {"profiles":[]} '''`, json: `{"profiles":[]}`, kind: RootBundle},
		{name: "triple double quoted", text: `""" {"profiles":[]} """`, json: `{"profiles":[]}`, kind: RootBundle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			document, err := DecodeText(tt.text)

			if err != nil {
				t.Fatalf("DecodeText() error = %v", err)
			}
			if string(document.JSON) != tt.json {
				t.Fatalf("JSON = %q, want %q", document.JSON, tt.json)
			}
			if document.Kind != tt.kind {
				t.Fatalf("Kind = %q, want %q", document.Kind, tt.kind)
			}
		})
	}
}

func TestDecodeRejectsMalformedAndOverLimitInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		code ErrorCode
	}{
		{name: "empty", text: " \n\t ", code: ERR_IMPORT_ENCODING},
		{name: "malformed JSON", text: `{"profiles":`, code: ERR_IMPORT_JSON},
		{name: "ambiguous root", text: `{"profiles":[],"inputs":[],"id":"synthetic"}`, code: ERR_IMPORT_ROOT},
		{name: "unknown root", text: `{"synthetic":true}`, code: ERR_IMPORT_ROOT},
		{name: "non-object root", text: `[]`, code: ERR_IMPORT_ROOT},
		{name: "unmatched fence", text: "```\n{\"profiles\":[]}", code: ERR_IMPORT_ENCODING},
		{name: "mismatched quote", text: `' {"profiles":[]} "`, code: ERR_IMPORT_ENCODING},
		{name: "mixed Base64 alphabets", text: "+_8=", code: ERR_IMPORT_ENCODING},
		{name: "Base64 padding in middle", text: "YW=Jj", code: ERR_IMPORT_ENCODING},
		{name: "Base64 excess padding", text: "YQ===", code: ERR_IMPORT_ENCODING},
		{name: "Base64 embedded whitespace", text: "YW Jj", code: ERR_IMPORT_ENCODING},
		{name: "Base64 embedded newline", text: "YW\nJj", code: ERR_IMPORT_ENCODING},
		{name: "Base64 invalid character", text: "YW$J", code: ERR_IMPORT_ENCODING},
		{name: "Base64 invalid Unicode", text: "YWéJ", code: ERR_IMPORT_ENCODING},
		{name: "Base64 modulo one length", text: "A", code: ERR_IMPORT_ENCODING},
		{name: "source over limit", text: string(validBundleOfSize(maxSourceSize + 1)), code: ERR_IMPORT_LIMIT_EXCEEDED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			document, err := DecodeText(tt.text)

			assertDecodeError(t, err, tt.code)
			assertZeroDocument(t, document)
		})
	}
}

func TestDecodeTextEntrypoints(t *testing.T) {
	t.Parallel()

	document, err := DecodeText(`{"profiles":[]}`)

	if err != nil {
		t.Fatalf("DecodeText() error = %v", err)
	}
	want := Document{JSON: []byte(`{"profiles":[]}`), Kind: RootBundle}
	if !reflect.DeepEqual(document, want) {
		t.Fatalf("DecodeText() = %#v, want %#v", document, want)
	}
}

func TestDecodeReaderEntrypoints(t *testing.T) {
	t.Parallel()

	source := []byte(`{"name":"synthetic","inputs":[]}`)
	document, err := DecodeReader(bytes.NewReader(source))

	if err != nil {
		t.Fatalf("DecodeReader() error = %v", err)
	}
	source[2] = 'X'
	if string(document.JSON) != `{"name":"synthetic","inputs":[]}` {
		t.Fatalf("JSON changed with caller input: %q", document.JSON)
	}
	if document.Kind != RootSingle {
		t.Fatalf("Kind = %q, want %q", document.Kind, RootSingle)
	}
}

func TestDecodeFileEntrypoints(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "synthetic.json")
	if err := os.WriteFile(path, []byte(`{"profiles":[]}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	document, err := DecodeFile(path)

	if err != nil {
		t.Fatalf("DecodeFile() error = %v", err)
	}
	if string(document.JSON) != `{"profiles":[]}` || document.Kind != RootBundle {
		t.Fatalf("DecodeFile() = %#v", document)
	}
}

type failingReader struct {
	cause error
	sent  bool
}

func (r *failingReader) Read(destination []byte) (int, error) {
	if r.sent {
		return 0, r.cause
	}
	r.sent = true
	return copy(destination, "{"), nil
}

func assertDecodeError(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want code %q", code)
	}
	var decodeErr *DecodeError
	if !errors.As(err, &decodeErr) {
		t.Fatalf("error type = %T, want *DecodeError", err)
	}
	if decodeErr.Code != code {
		t.Fatalf("error code = %q, want %q", decodeErr.Code, code)
	}
}

func assertZeroDocument(t *testing.T, document Document) {
	t.Helper()
	if !reflect.DeepEqual(document, Document{}) {
		t.Fatalf("document = %#v, want zero Document", document)
	}
}

var _ io.Reader = (*failingReader)(nil)
