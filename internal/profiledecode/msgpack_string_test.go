package profiledecode

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func TestDecodeMsgpackString_WhenEachSupportedTag_ReturnsExactJSON(t *testing.T) {
	t.Parallel()

	bundle := []byte(`{"profiles":[]}`)
	tests := []struct {
		name string
		tag  byte
	}{
		{name: "fixstr", tag: 0xa0},
		{name: "str8", tag: 0xd9},
		{name: "str16", tag: 0xda},
		{name: "str32", tag: 0xdb},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			compressed := mustCompressLZMA(t, msgpackString(t, tt.tag, bundle), false)

			document, err := DecodeReader(bytes.NewReader(compressed))

			if err != nil {
				t.Fatalf("DecodeReader() error = %v", err)
			}
			if !bytes.Equal(document.JSON, bundle) {
				t.Fatalf("JSON = %q, want %q", document.JSON, bundle)
			}
			if document.Kind != RootBundle {
				t.Fatalf("Kind = %q, want %q", document.Kind, RootBundle)
			}
		})
	}
}

func TestDecodeMsgpackString_WhenUnsupportedType_ReturnsTypeError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		envelope []byte
	}{
		{name: "map", envelope: []byte{0x80}},
		{name: "binary", envelope: []byte{0xc4, 0x00}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			document, err := decodeSyntheticEnvelope(t, tt.envelope)

			assertDecodeError(t, err, ERR_IMPORT_MSGPACK_TYPE)
			assertZeroDocument(t, document)
		})
	}
}

func TestDecodeMsgpackString_WhenHeaderTruncated_ReturnsLengthMismatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		envelope []byte
	}{
		{name: "missing tag", envelope: nil},
		{name: "str8 missing length", envelope: []byte{0xd9}},
		{name: "str16 missing length", envelope: []byte{0xda}},
		{name: "str16 partial length", envelope: []byte{0xda, 0x00}},
		{name: "str32 missing length", envelope: []byte{0xdb}},
		{name: "str32 one length byte", envelope: []byte{0xdb, 0x00}},
		{name: "str32 two length bytes", envelope: []byte{0xdb, 0x00, 0x00}},
		{name: "str32 three length bytes", envelope: []byte{0xdb, 0x00, 0x00, 0x00}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			document, err := decodeSyntheticEnvelope(t, tt.envelope)

			assertDecodeError(t, err, ERR_IMPORT_LENGTH_MISMATCH)
			assertZeroDocument(t, document)
		})
	}
}

func TestDecodeMsgpackString_WhenLengthMismatch_ReturnsZeroDocument(t *testing.T) {
	t.Parallel()

	uint32Maximum := []byte{0xdb, 0xff, 0xff, 0xff, 0xff, '{'}
	tests := []struct {
		name     string
		envelope []byte
	}{
		{name: "fixstr short payload", envelope: []byte{0xa2, '{'}},
		{name: "str8 short payload", envelope: []byte{0xd9, 0x02, '{'}},
		{name: "str16 short payload", envelope: []byte{0xda, 0x00, 0x02, '{'}},
		{name: "str32 short payload", envelope: []byte{0xdb, 0x00, 0x00, 0x00, 0x02, '{'}},
		{name: "uint32 maximum against tiny input", envelope: uint32Maximum},
		{name: "fixstr trailing payload", envelope: []byte{0xa1, '{', '}'}},
		{name: "str8 trailing payload", envelope: []byte{0xd9, 0x01, '{', '}'}},
		{name: "str16 trailing payload", envelope: []byte{0xda, 0x00, 0x01, '{', '}'}},
		{name: "str32 trailing payload", envelope: []byte{0xdb, 0x00, 0x00, 0x00, 0x01, '{', '}'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			document, err := decodeSyntheticEnvelope(t, tt.envelope)

			assertDecodeError(t, err, ERR_IMPORT_LENGTH_MISMATCH)
			assertZeroDocument(t, document)
		})
	}
}

func TestDecodeMsgpackString_WhenPayloadInvalidUTF8_ReturnsUTF8Error(t *testing.T) {
	t.Parallel()

	document, err := decodeSyntheticEnvelope(t, msgpackString(t, 0xd9, []byte{0xff}))

	assertDecodeError(t, err, ERR_IMPORT_UTF8)
	assertZeroDocument(t, document)
}

func decodeSyntheticEnvelope(t *testing.T, envelope []byte) (Document, error) {
	t.Helper()
	return DecodeReader(bytes.NewReader(mustCompressLZMA(t, envelope, false)))
}

func msgpackString(t *testing.T, tag byte, payload []byte) []byte {
	t.Helper()

	var header []byte
	switch tag {
	case 0xa0:
		if len(payload) > 31 {
			t.Fatalf("fixstr payload length = %d", len(payload))
		}
		header = []byte{0xa0 | byte(len(payload))}
	case 0xd9:
		if len(payload) > math.MaxUint8 {
			t.Fatalf("str8 payload length = %d", len(payload))
		}
		header = []byte{0xd9, byte(len(payload))}
	case 0xda:
		if len(payload) > math.MaxUint16 {
			t.Fatalf("str16 payload length = %d", len(payload))
		}
		header = make([]byte, 3)
		header[0] = 0xda
		binary.BigEndian.PutUint16(header[1:], uint16(len(payload)))
	case 0xdb:
		header = make([]byte, 5)
		header[0] = 0xdb
		binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
	default:
		t.Fatalf("unsupported synthetic tag 0x%x", tag)
	}
	return append(header, payload...)
}
