package profiledecode

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

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

func TestDecodeTextEntrypoints_WhenInputsAreEquivalent_ReturnSameDocuments(t *testing.T) {
	t.Parallel()

	// Given
	tests := []struct {
		name   string
		source []byte
	}{
		{name: "raw JSON", source: []byte(`{"profiles":[]}`)},
		{name: "encoded envelope", source: []byte(encodeSyntheticEnvelopeText(t, msgpackString(t, 0xda, []byte(`{"profiles":[]}`))))},
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
		})
	}
}

func TestDecodeReaderEntrypoints_WhenSourceExceedsCap_ReturnsLimitAndZeroDocument(t *testing.T) {
	t.Parallel()

	// Given / When
	document, err := DecodeReader(strings.NewReader(string(validBundleOfSize(maxSourceSize + 1))))

	// Then
	assertDecodeError(t, err, ERR_IMPORT_LIMIT_EXCEEDED)
	assertZeroDocument(t, document)
}

func TestDecodeReaderEntrypoints_WhenReadFailsAfterPartialBytes_PreservesCauseAndReturnsZeroDocument(t *testing.T) {
	t.Parallel()

	// Given
	cause := errors.New("synthetic read failure")

	// When
	document, err := DecodeReader(&failingReader{cause: cause})

	// Then
	assertDecodeError(t, err, ERR_IMPORT_ENCODING)
	if !errors.Is(err, cause) {
		t.Fatalf("error = %v, want wrapped read cause", err)
	}
	assertZeroDocument(t, document)
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

func TestDecodeFileEntrypoints_WhenDecodeReturns_ClosesDescriptor(t *testing.T) {
	// Given
	directory := t.TempDir()
	path := filepath.Join(directory, "export.json")
	renamedPath := filepath.Join(directory, "decoded.json")
	if err := os.WriteFile(path, []byte(`{"profiles":[]}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// When
	_, err := DecodeFile(path)

	// Then
	if err != nil {
		t.Fatalf("DecodeFile() error = %v", err)
	}
	if err := os.Rename(path, renamedPath); err != nil {
		t.Fatalf("Rename() after DecodeFile() error = %v", err)
	}
	if err := os.Remove(renamedPath); err != nil {
		t.Fatalf("Remove() after DecodeFile() error = %v", err)
	}
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
