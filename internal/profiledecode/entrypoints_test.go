package profiledecode

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDecodeTextEntrypoints_WhenInputsAreEquivalent_ReturnSameDocuments(t *testing.T) {
	t.Parallel()

	// Given
	tests := []struct {
		name   string
		source []byte
		kind   RootKind
	}{
		{name: "raw bundle JSON", source: []byte(`{"profiles":[]}`), kind: RootBundle},
		{name: "raw single JSON", source: []byte(`{"name":"synthetic","inputs":[]}`), kind: RootSingle},
		{name: "encoded envelope", source: []byte(encodeSyntheticEnvelopeText(t, msgpackString(t, 0xda, []byte(`{"profiles":[]}`)))), kind: RootBundle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "export.txt")
			if err := os.WriteFile(path, tt.source, 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}

			// When
			fromText, textErr := DecodeText(string(tt.source))
			fromReader, readerErr := DecodeReader(bytes.NewReader(tt.source))
			fromFile, fileErr := DecodeFile(path)

			// Then
			if textErr != nil || readerErr != nil || fileErr != nil {
				t.Fatalf("entrypoint errors = text:%v reader:%v file:%v", textErr, readerErr, fileErr)
			}
			if !reflect.DeepEqual(fromReader, fromText) || !reflect.DeepEqual(fromFile, fromText) {
				t.Fatalf("documents differ: text=%#v reader=%#v file=%#v", fromText, fromReader, fromFile)
			}
			if fromText.Kind != tt.kind {
				t.Fatalf("Kind = %q, want %q", fromText.Kind, tt.kind)
			}
		})
	}
}

func TestDecodeFileEntrypoints_WhenPathDoesNotExist_PreservesCauseAndReturnsZeroDocument(t *testing.T) {
	t.Parallel()

	// Given / When
	document, err := DecodeFile(filepath.Join(t.TempDir(), "missing.json"))

	// Then
	assertDecodeError(t, err, ERR_IMPORT_ENCODING)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want os.ErrNotExist", err)
	}
	assertZeroDocument(t, document)
}

func TestDecodeTextEntrypoints_WhenInputIsMalformed_ReturnSameErrorAndZeroDocument(t *testing.T) {
	t.Parallel()

	// Given
	source := []byte(`{"profiles":`)
	path := filepath.Join(t.TempDir(), "malformed.json")
	if err := os.WriteFile(path, source, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// When
	fromText, textErr := DecodeText(string(source))
	fromReader, readerErr := DecodeReader(bytes.NewReader(source))
	fromFile, fileErr := DecodeFile(path)

	// Then
	assertDecodeError(t, textErr, ERR_IMPORT_JSON)
	assertDecodeError(t, readerErr, ERR_IMPORT_JSON)
	assertDecodeError(t, fileErr, ERR_IMPORT_JSON)
	assertZeroDocument(t, fromText)
	assertZeroDocument(t, fromReader)
	assertZeroDocument(t, fromFile)
}

func TestDecodeReaderEntrypoints_WhenCallerMutatesSource_ReturnedJSONDoesNotAliasInput(t *testing.T) {
	t.Parallel()

	// Given
	source := []byte(`{"profiles":[]}`)

	// When
	document, err := DecodeReader(bytes.NewReader(source))

	// Then
	if err != nil {
		t.Fatalf("DecodeReader() error = %v", err)
	}
	copy(source, `{"profiles":{}}`)
	if string(document.JSON) != `{"profiles":[]}` {
		t.Fatalf("JSON changed with caller mutation: %q", document.JSON)
	}
}
