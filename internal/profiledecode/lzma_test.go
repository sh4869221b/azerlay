package profiledecode

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"io"
	"testing"

	"github.com/ulikunitz/xz/lzma"
)

func TestDecodeLZMA_WhenCompleteKnownSize_ReturnsEveryByte(t *testing.T) {
	t.Parallel()

	payload := []byte("synthetic known-size payload")
	compressed := mustCompressLZMA(t, payload, true)

	got, err := decodeLZMA(compressed)

	if err != nil {
		t.Fatalf("decodeLZMA() error = %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("decodeLZMA() = %q, want %q", got, payload)
	}
}

func TestDecodeLZMA_WhenCompleteUnknownSize_ReturnsEveryByte(t *testing.T) {
	t.Parallel()

	payload := []byte("synthetic unknown-size payload")
	compressed := mustCompressLZMA(t, payload, false)

	got, err := decodeLZMA(compressed)

	if err != nil {
		t.Fatalf("decodeLZMA() error = %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("decodeLZMA() = %q, want %q", got, payload)
	}
}

func TestDecodeLZMA_WhenComplete_AcceptsExactOutputLimit(t *testing.T) {
	compressed := mustCompressLZMARepeated(t, maxOutputSize, false)

	got, err := decodeLZMA(compressed)

	if err != nil {
		t.Fatalf("decodeLZMA() error = %v", err)
	}
	if len(got) != maxOutputSize {
		t.Fatalf("decoded length = %d, want %d", len(got), maxOutputSize)
	}
}

func TestDecodeLZMA_WhenComplete_AcceptsExactDictionaryLimit(t *testing.T) {
	t.Parallel()

	compressed := mustCompressLZMA(t, []byte("dictionary boundary"), true)
	binary.LittleEndian.PutUint32(compressed[1:5], maxDictionarySize)

	got, err := decodeLZMA(compressed)

	if err != nil {
		t.Fatalf("decodeLZMA() error = %v", err)
	}
	if string(got) != "dictionary boundary" {
		t.Fatalf("decodeLZMA() = %q", got)
	}
}

func TestDecodeLZMA_WhenDictionaryDeclarationIsNoncanonicalAndBounded_AcceptsPublicForms(t *testing.T) {
	t.Parallel()

	const bundle = `{"profiles":[]}`
	compressed := mustCompressLZMA(t, msgpackString(t, 0xda, []byte(bundle)), true)
	binary.LittleEndian.PutUint32(compressed[1:5], lzma.MinDictCap+1)
	tests := []struct {
		name   string
		decode func() (Document, error)
	}{
		{name: "raw reader", decode: func() (Document, error) { return DecodeReader(bytes.NewReader(compressed)) }},
		{name: "Base64URL text", decode: func() (Document, error) {
			return DecodeText(base64.RawURLEncoding.EncodeToString(compressed))
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document, err := tt.decode()

			if err != nil {
				t.Fatalf("decode() error = %v", err)
			}
			if string(document.JSON) != bundle {
				t.Fatalf("JSON = %q, want %q", document.JSON, bundle)
			}
			if document.Kind != RootBundle {
				t.Fatalf("kind = %q, want %q", document.Kind, RootBundle)
			}
		})
	}
}

func TestDecodeLZMA_WhenCorrupt_RejectsMalformedOrIncompleteStream(t *testing.T) {
	t.Parallel()

	complete := mustCompressLZMA(t, []byte("payload emitted before terminal failure"), false)
	wrongSize := append([]byte(nil), complete...)
	binary.LittleEndian.PutUint64(wrongSize[5:13], uint64(len("payload emitted before terminal failure")+1))
	exactInput := append([]byte(nil), complete...)
	exactInput = append(exactInput, make([]byte, maxCompressedSize-len(exactInput))...)
	tests := []struct {
		name       string
		compressed []byte
		code       ErrorCode
	}{
		{name: "short header", compressed: []byte{0x5d}, code: ERR_IMPORT_LZMA_HEADER},
		{name: "invalid property", compressed: append([]byte{0xff}, make([]byte, 12)...), code: ERR_IMPORT_LZMA_HEADER},
		{name: "truncated intact header", compressed: append([]byte(nil), complete[:len(complete)-1]...), code: ERR_IMPORT_LZMA_CORRUPT},
		{name: "wrong known size", compressed: wrongSize, code: ERR_IMPORT_LZMA_CORRUPT},
		{name: "trailing compressed byte", compressed: append(append([]byte(nil), complete...), 0), code: ERR_IMPORT_LZMA_CORRUPT},
		{name: "exact compressed cap reaches decoder", compressed: exactInput, code: ERR_IMPORT_LZMA_CORRUPT},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeLZMA(tt.compressed)

			assertDecodeError(t, err, tt.code)
			if got != nil {
				t.Fatalf("decodeLZMA() returned %d partial bytes", len(got))
			}
		})
	}
}

func TestDecodeLZMA_WhenLimitExceeded_RejectsBeforeAdoption(t *testing.T) {
	complete := mustCompressLZMA(t, []byte("bounded payload"), false)
	overDictionary := append([]byte(nil), complete...)
	binary.LittleEndian.PutUint32(overDictionary[1:5], maxDictionarySize+1)
	overKnownSize := append([]byte(nil), complete...)
	binary.LittleEndian.PutUint64(overKnownSize[5:13], uint64(maxOutputSize)+1)
	overCompressed := append([]byte(nil), complete...)
	overCompressed = append(overCompressed, make([]byte, maxCompressedSize+1-len(overCompressed))...)
	overOutput := mustCompressLZMARepeated(t, maxOutputSize+1, false)
	tests := []struct {
		name       string
		compressed []byte
	}{
		{name: "dictionary 64 MiB plus one", compressed: overDictionary},
		{name: "known output 64 MiB plus one", compressed: overKnownSize},
		{name: "compressed input 8 MiB plus one", compressed: overCompressed},
		{name: "unknown output 64 MiB plus one", compressed: overOutput},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeLZMA(tt.compressed)

			assertDecodeError(t, err, ERR_IMPORT_LIMIT_EXCEEDED)
			if got != nil {
				t.Fatalf("decodeLZMA() returned %d partial bytes", len(got))
			}
		})
	}
}

func TestDetectOuter_WhenRawLZMAHeader_RoutesReaderToLZMA(t *testing.T) {
	t.Parallel()

	compressed := mustCompressLZMA(t, []byte{0xd9, 0}, false)
	if !hasLZMAHeader(compressed) {
		t.Fatal("hasLZMAHeader() = false for complete LZMA-Alone stream")
	}

	document, err := DecodeReader(bytes.NewReader(compressed))

	assertDecodeError(t, err, ERR_IMPORT_JSON)
	assertZeroDocument(t, document)
}

func TestDetectOuter_WhenRawLZMAHeader_RejectsInvalidProperty(t *testing.T) {
	t.Parallel()

	header := append([]byte{0xff}, make([]byte, 12)...)

	if hasLZMAHeader(header) {
		t.Fatal("hasLZMAHeader() = true for invalid property")
	}
}

func TestDecodeLZMA_WhenBase64Decoded_RoutesThroughSameDecoder(t *testing.T) {
	t.Parallel()

	compressed := mustCompressLZMA(t, []byte{0xd9, 0}, false)
	text := base64.RawURLEncoding.EncodeToString(compressed)

	document, err := DecodeText(text)

	assertDecodeError(t, err, ERR_IMPORT_JSON)
	assertZeroDocument(t, document)
}

func mustCompressLZMA(t *testing.T, payload []byte, knownSize bool) []byte {
	t.Helper()

	var destination bytes.Buffer
	config := lzma.WriterConfig{DictCap: lzma.MinDictCap, SizeInHeader: knownSize, EOSMarker: true}
	if knownSize {
		config.Size = int64(len(payload))
	}
	writer, err := config.NewWriter(&destination)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	if _, err := writer.Write(payload); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return destination.Bytes()
}

func mustCompressLZMARepeated(t *testing.T, size int, knownSize bool) []byte {
	t.Helper()

	var destination bytes.Buffer
	config := lzma.WriterConfig{DictCap: lzma.MinDictCap, SizeInHeader: knownSize, EOSMarker: true}
	if knownSize {
		config.Size = int64(size)
	}
	writer, err := config.NewWriter(&destination)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	if _, err := io.CopyN(writer, zeroReader{}, int64(size)); err != nil {
		t.Fatalf("CopyN() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return destination.Bytes()
}

type zeroReader struct{}

func (zeroReader) Read(destination []byte) (int, error) {
	clear(destination)
	return len(destination), nil
}
