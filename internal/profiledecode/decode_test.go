package profiledecode

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
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

	bundle := []byte(`{"profiles":[]}`)
	single := []byte(`{"id":0,"inputs":[]}`)
	rawLZMA := mustCompressLZMA(t, msgpackString(t, 0xa0, bundle), false)
	urlPadded, urlPaddedJSON := mustEncodedEnvelope(t, bundle, syntheticEncoding{tag: 0xd9, codec: base64.URLEncoding, padded: true})
	standardPadded, standardPaddedJSON := mustEncodedEnvelope(t, single, syntheticEncoding{tag: 0xdb, codec: base64.StdEncoding, padded: true})
	standardUnpadded, standardUnpaddedJSON := mustEncodedEnvelope(t, single, syntheticEncoding{tag: 0xd9, codec: base64.RawStdEncoding})
	wrapped := []struct {
		name   string
		decode func() (Document, error)
		json   []byte
		kind   RootKind
	}{
		{name: "raw LZMA fixstr bundle", decode: func() (Document, error) { return DecodeReader(bytes.NewReader(rawLZMA)) }, json: bundle, kind: RootBundle},
		{name: "Base64URL padded str8 bundle", decode: func() (Document, error) { return DecodeText(urlPadded) }, json: urlPaddedJSON, kind: RootBundle},
		{name: "Base64URL unpadded str16 bundle", decode: func() (Document, error) {
			return DecodeText(base64.RawURLEncoding.EncodeToString(mustCompressLZMA(t, msgpackString(t, 0xda, bundle), false)))
		}, json: bundle, kind: RootBundle},
		{name: "standard Base64 padded str32 single", decode: func() (Document, error) { return DecodeText(standardPadded) }, json: standardPaddedJSON, kind: RootSingle},
		{name: "standard Base64 unpadded str8 single", decode: func() (Document, error) { return DecodeText(standardUnpadded) }, json: standardUnpaddedJSON, kind: RootSingle},
	}
	for _, tt := range wrapped {
		t.Run(tt.name, func(t *testing.T) {
			document, err := tt.decode()

			if err != nil {
				t.Fatalf("decode() error = %v", err)
			}
			if !bytes.Equal(document.JSON, tt.json) {
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

	completeLZMA := mustCompressLZMA(t, []byte("partial output must not be adopted"), false)
	invalidProperty := append([]byte(nil), completeLZMA...)
	invalidProperty[0] = 0xff
	overDictionary := append([]byte(nil), completeLZMA...)
	binary.LittleEndian.PutUint32(overDictionary[1:5], 128<<20)
	overKnownSize := append([]byte(nil), completeLZMA...)
	binary.LittleEndian.PutUint64(overKnownSize[5:13], uint64(maxOutputSize)+1)
	wrongKnownSize := append([]byte(nil), completeLZMA...)
	binary.LittleEndian.PutUint64(wrongKnownSize[5:13], uint64(len("partial output must not be adopted")+1))
	overCompressed := append([]byte(nil), completeLZMA...)
	overCompressed = append(overCompressed, make([]byte, maxCompressedSize+1-len(overCompressed))...)
	overOutput := mustCompressLZMARepeated(t, maxOutputSize+1, false)
	invalidUTF8 := encodeLZMATestText(mustCompressLZMA(t, msgpackString(t, 0xd9, []byte{0xff}), false))
	malformedEnvelopeJSON := encodeSyntheticEnvelopeText(t, msgpackString(t, 0xd9, []byte(`{"profiles":`)))
	trailingEnvelopeJSON := encodeSyntheticEnvelopeText(t, msgpackString(t, 0xd9, []byte(`{"profiles":[]} true`)))
	ambiguousEnvelopeRoot := encodeSyntheticEnvelopeText(t, msgpackString(t, 0xd9, []byte(`{"profiles":[],"inputs":[],"id":0}`)))
	completeStr32 := mustCompressLZMA(t, msgpackString(t, 0xdb, []byte(`{"profiles":[]}`)), false)
	corruptStr32 := encodeLZMATestText(completeStr32[:len(completeStr32)-1])

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
		{name: "LZMA malformed property", text: encodeLZMATestText(invalidProperty), code: ERR_IMPORT_LZMA_HEADER},
		{name: "LZMA dictionary over limit", text: encodeLZMATestText(overDictionary), code: ERR_IMPORT_LIMIT_EXCEEDED},
		{name: "LZMA known output over limit", text: encodeLZMATestText(overKnownSize), code: ERR_IMPORT_LIMIT_EXCEEDED},
		{name: "LZMA compressed input over limit", text: encodeLZMATestText(overCompressed), code: ERR_IMPORT_LIMIT_EXCEEDED},
		{name: "LZMA unknown output over limit", text: encodeLZMATestText(overOutput), code: ERR_IMPORT_LIMIT_EXCEEDED},
		{name: "LZMA truncated intact header", text: encodeLZMATestText(completeLZMA[:len(completeLZMA)-1]), code: ERR_IMPORT_LZMA_CORRUPT},
		{name: "LZMA wrong known size", text: encodeLZMATestText(wrongKnownSize), code: ERR_IMPORT_LZMA_CORRUPT},
		{name: "LZMA trailing compressed byte", text: encodeLZMATestText(append(append([]byte(nil), completeLZMA...), 0)), code: ERR_IMPORT_LZMA_CORRUPT},
		{name: "MessagePack map", text: encodeSyntheticEnvelopeText(t, []byte{0x80}), code: ERR_IMPORT_MSGPACK_TYPE},
		{name: "MessagePack binary", text: encodeSyntheticEnvelopeText(t, []byte{0xc4, 0}), code: ERR_IMPORT_MSGPACK_TYPE},
		{name: "MessagePack missing tag", text: encodeSyntheticEnvelopeText(t, nil), code: ERR_IMPORT_LENGTH_MISMATCH},
		{name: "MessagePack str8 missing length", text: encodeSyntheticEnvelopeText(t, []byte{0xd9}), code: ERR_IMPORT_LENGTH_MISMATCH},
		{name: "MessagePack str16 missing length", text: encodeSyntheticEnvelopeText(t, []byte{0xda}), code: ERR_IMPORT_LENGTH_MISMATCH},
		{name: "MessagePack str16 partial length", text: encodeSyntheticEnvelopeText(t, []byte{0xda, 0}), code: ERR_IMPORT_LENGTH_MISMATCH},
		{name: "MessagePack str32 missing length", text: encodeSyntheticEnvelopeText(t, []byte{0xdb}), code: ERR_IMPORT_LENGTH_MISMATCH},
		{name: "MessagePack str32 one length byte", text: encodeSyntheticEnvelopeText(t, []byte{0xdb, 0}), code: ERR_IMPORT_LENGTH_MISMATCH},
		{name: "MessagePack str32 two length bytes", text: encodeSyntheticEnvelopeText(t, []byte{0xdb, 0, 0}), code: ERR_IMPORT_LENGTH_MISMATCH},
		{name: "MessagePack str32 three length bytes", text: encodeSyntheticEnvelopeText(t, []byte{0xdb, 0, 0, 0}), code: ERR_IMPORT_LENGTH_MISMATCH},
		{name: "MessagePack uint32 maximum", text: encodeSyntheticEnvelopeText(t, []byte{0xdb, 0xff, 0xff, 0xff, 0xff, '{'}), code: ERR_IMPORT_LENGTH_MISMATCH},
		{name: "MessagePack short payload", text: encodeSyntheticEnvelopeText(t, []byte{0xd9, 2, '{'}), code: ERR_IMPORT_LENGTH_MISMATCH},
		{name: "MessagePack trailing payload", text: encodeSyntheticEnvelopeText(t, []byte{0xd9, 1, '{', '}'}), code: ERR_IMPORT_LENGTH_MISMATCH},
		{name: "invalid UTF-8", text: invalidUTF8, code: ERR_IMPORT_UTF8},
		{name: "malformed envelope JSON", text: malformedEnvelopeJSON, code: ERR_IMPORT_JSON},
		{name: "trailing envelope JSON", text: trailingEnvelopeJSON, code: ERR_IMPORT_JSON},
		{name: "ambiguous envelope root", text: ambiguousEnvelopeRoot, code: ERR_IMPORT_ROOT},
		{name: "corrupt str32 equivalent", text: corruptStr32, code: ERR_IMPORT_LZMA_CORRUPT},
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

func TestDecodeText_WhenSampleEquivalentStr16_ReturnsExactOwnedBundle(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"profiles":[],"version":"synthetic"}`)
	envelope := msgpackString(t, 0xda, payload)
	text := base64.RawURLEncoding.EncodeToString(mustCompressLZMA(t, envelope, false))

	document, err := DecodeText(text)

	if err != nil {
		t.Fatalf("DecodeText() error = %v", err)
	}
	if !bytes.Equal(document.JSON, payload) {
		t.Fatalf("JSON = %q, want %q", document.JSON, payload)
	}
	if document.Kind != RootBundle {
		t.Fatalf("Kind = %q, want %q", document.Kind, RootBundle)
	}
	payload[2] = 'X'
	if string(document.JSON) != `{"profiles":[],"version":"synthetic"}` {
		t.Fatalf("JSON changed with fixture mutation: %q", document.JSON)
	}
}

func TestDecodeText_WhenCorruptStr32Equivalent_ReturnsNoDocument(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"profiles":[]}`)
	compressed := mustCompressLZMA(t, msgpackString(t, 0xdb, payload), false)
	text := base64.RawURLEncoding.EncodeToString(compressed[:len(compressed)-1])

	document, err := DecodeText(text)

	assertDecodeError(t, err, ERR_IMPORT_LZMA_CORRUPT)
	assertZeroDocument(t, document)
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
