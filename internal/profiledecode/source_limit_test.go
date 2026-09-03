package profiledecode

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceLimit_WhenConstantsAreDeclared_HasExactValues(t *testing.T) {
	t.Parallel()

	if maxSourceSize != 16<<20 {
		t.Fatalf("maxSourceSize = %d, want %d", maxSourceSize, 16<<20)
	}
	if maxCompressedSize != 8<<20 {
		t.Fatalf("maxCompressedSize = %d, want %d", maxCompressedSize, 8<<20)
	}
	if maxDictionarySize != 64<<20 {
		t.Fatalf("maxDictionarySize = %d, want %d", maxDictionarySize, 64<<20)
	}
	if maxOutputSize != 64<<20 {
		t.Fatalf("maxOutputSize = %d, want %d", maxOutputSize, 64<<20)
	}
}

func TestSourceLimit_WhenTextIsExactLimit_AcceptsIt(t *testing.T) {
	t.Parallel()

	source := validBundleOfSize(maxSourceSize)
	document, err := DecodeText(string(source))

	if err != nil {
		t.Fatalf("DecodeText() error = %v", err)
	}
	if !bytes.Equal(document.JSON, source) {
		t.Fatalf("JSON length = %d, want %d", len(document.JSON), len(source))
	}
}

func TestSourceLimit_WhenTextExceedsLimit_RejectsIt(t *testing.T) {
	t.Parallel()

	document, err := DecodeText(string(validBundleOfSize(maxSourceSize + 1)))

	assertDecodeError(t, err, ERR_IMPORT_LIMIT_EXCEEDED)
	assertZeroDocument(t, document)
}

func TestSourceLimit_WhenReaderIsExactLimit_AcceptsIt(t *testing.T) {
	t.Parallel()

	source := validBundleOfSize(maxSourceSize)
	document, err := DecodeReader(bytes.NewReader(source))

	if err != nil {
		t.Fatalf("DecodeReader() error = %v", err)
	}
	if !bytes.Equal(document.JSON, source) {
		t.Fatalf("JSON length = %d, want %d", len(document.JSON), len(source))
	}
}

func TestSourceLimit_WhenReaderExceedsLimit_RejectsIt(t *testing.T) {
	t.Parallel()

	document, err := DecodeReader(strings.NewReader(string(validBundleOfSize(maxSourceSize + 1))))

	assertDecodeError(t, err, ERR_IMPORT_LIMIT_EXCEEDED)
	assertZeroDocument(t, document)
}

func TestSourceLimit_WhenFileIsExactLimit_AcceptsIt(t *testing.T) {
	t.Parallel()

	source := validBundleOfSize(maxSourceSize)
	path := filepath.Join(t.TempDir(), "exact-limit.json")
	if err := os.WriteFile(path, source, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	document, err := DecodeFile(path)

	if err != nil {
		t.Fatalf("DecodeFile() error = %v", err)
	}
	if !bytes.Equal(document.JSON, source) {
		t.Fatalf("JSON length = %d, want %d", len(document.JSON), len(source))
	}
}

func TestSourceLimit_WhenFileExceedsLimit_RejectsIt(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "over-limit.json")
	if err := os.WriteFile(path, validBundleOfSize(maxSourceSize+1), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	document, err := DecodeFile(path)

	assertDecodeError(t, err, ERR_IMPORT_LIMIT_EXCEEDED)
	assertZeroDocument(t, document)
}

func validBundleOfSize(size int) []byte {
	const prefix = `{"profiles":[],"padding":"`
	const suffix = `"}`
	return []byte(prefix + strings.Repeat("x", size-len(prefix)-len(suffix)) + suffix)
}
