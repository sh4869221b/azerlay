package profiledecode

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestDecodeError_WhenDecodeFails_HasStableCodeAndTypedShape(t *testing.T) {
	t.Parallel()

	document, err := DecodeText(`{"profiles":`)

	assertZeroDocument(t, document)
	assertDecodeError(t, err, ERR_IMPORT_JSON)
}

func TestDecodeError_WhenReaderFails_RetainsCauseWithoutDisclosingSentinel(t *testing.T) {
	t.Parallel()

	privateCause := errors.New("private-source-sentinel")
	document, err := DecodeReader(&failingReader{cause: privateCause})

	assertZeroDocument(t, document)
	assertDecodeError(t, err, ERR_IMPORT_ENCODING)
	if !errors.Is(err, privateCause) {
		t.Fatal("reader cause is not retained")
	}
	if strings.Contains(err.Error(), privateCause.Error()) {
		t.Fatalf("Error() disclosed private sentinel: %q", err)
	}
}

func TestDecodeError_WhenFileOpenFails_RetainsIOCause(t *testing.T) {
	t.Parallel()

	document, err := DecodeFile("missing-synthetic-file.json")

	assertZeroDocument(t, document)
	assertDecodeError(t, err, ERR_IMPORT_ENCODING)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file-open cause = %v, want os.ErrNotExist", err)
	}
}

func TestDecodeError_WhenMalformedInputContainsPrivateText_DoesNotDiscloseIt(t *testing.T) {
	t.Parallel()

	const privateText = "private-input-sentinel"
	document, err := DecodeText(`{"profiles":` + privateText)

	assertZeroDocument(t, document)
	assertDecodeError(t, err, ERR_IMPORT_JSON)
	if strings.Contains(err.Error(), privateText) {
		t.Fatalf("Error() disclosed input: %q", err)
	}
}

func TestDecodeError_WhenCodesAreDeclared_UsesExactMachineValues(t *testing.T) {
	t.Parallel()

	codes := []ErrorCode{
		ERR_IMPORT_ENCODING,
		ERR_IMPORT_LZMA_HEADER,
		ERR_IMPORT_LZMA_CORRUPT,
		ERR_IMPORT_MSGPACK_TYPE,
		ERR_IMPORT_LENGTH_MISMATCH,
		ERR_IMPORT_UTF8,
		ERR_IMPORT_JSON,
		ERR_IMPORT_ROOT,
		ERR_IMPORT_LIMIT_EXCEEDED,
	}
	want := []string{
		"ERR_IMPORT_ENCODING",
		"ERR_IMPORT_LZMA_HEADER",
		"ERR_IMPORT_LZMA_CORRUPT",
		"ERR_IMPORT_MSGPACK_TYPE",
		"ERR_IMPORT_LENGTH_MISMATCH",
		"ERR_IMPORT_UTF8",
		"ERR_IMPORT_JSON",
		"ERR_IMPORT_ROOT",
		"ERR_IMPORT_LIMIT_EXCEEDED",
	}

	for index, code := range codes {
		if string(code) != want[index] {
			t.Fatalf("code[%d] = %q, want %q", index, code, want[index])
		}
	}
}
