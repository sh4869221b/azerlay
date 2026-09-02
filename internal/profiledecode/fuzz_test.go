package profiledecode

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ulikunitz/xz/lzma"
)

func FuzzNormalizeOuterText(f *testing.F) {
	for _, seed := range []string{
		`{"profiles":[]}`,
		" \ufeff```{\"profiles\":[]} ``` ",
		`"{"inputs":[],"name":"synthetic"}"`,
		"```unterminated",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > maxSourceSize {
			return
		}

		first, firstErr := normalizeOuterText(text)
		second, secondErr := normalizeOuterText(text)

		if first != second || (firstErr == nil) != (secondErr == nil) {
			t.Fatalf("normalization is nondeterministic: first=(%q, %v), second=(%q, %v)", first, firstErr, second, secondErr)
		}
		if firstErr != nil {
			if first != "" || !errors.Is(firstErr, errMalformedOuterWrapper) {
				t.Fatalf("failed normalization returned output=%q error=%v", first, firstErr)
			}
			return
		}
		if len(first) > len(text) {
			t.Fatalf("normalized length = %d, input length = %d", len(first), len(text))
		}
		if first != strings.TrimSpace(first) {
			t.Fatalf("normalization retained outer whitespace: %q", first)
		}
	})
}

func FuzzDecodeBase64(f *testing.F) {
	f.Add("-w==")
	f.Add("-w")
	f.Add("+w==")
	f.Add("+w")
	f.Add("A")
	f.Add(`{"profiles":[]}`)

	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > maxSourceSize {
			return
		}
		form, err := detectOuter(text)
		if err != nil {
			return
		}

		first, firstErr := decodeBase64(text, form)
		second, secondErr := decodeBase64(text, form)
		firstCode, firstFailed := fuzzDecodeCode(t, firstErr)
		secondCode, secondFailed := fuzzDecodeCode(t, secondErr)

		if firstFailed != secondFailed || firstCode != secondCode || !bytes.Equal(first, second) {
			t.Fatalf("Base64 decode is nondeterministic: codes=(%q,%q), lengths=(%d,%d)", firstCode, secondCode, len(first), len(second))
		}
		if firstFailed {
			if first != nil || second != nil {
				t.Fatalf("failed Base64 decode returned bytes: lengths=(%d,%d)", len(first), len(second))
			}
			return
		}
		if len(first) > maxCompressedSize {
			t.Fatalf("decoded length = %d, limit = %d", len(first), maxCompressedSize)
		}
		codec := base64.URLEncoding
		if form == outerBase64StandardPadded || form == outerBase64StandardUnpadded {
			codec = base64.StdEncoding
		}
		padded := text + strings.Repeat("=", (4-len(text)%4)%4)
		want, referenceErr := codec.DecodeString(padded)
		if referenceErr != nil {
			t.Fatalf("accepted Base64 is rejected by reference decoder: %v", referenceErr)
		}
		if !bytes.Equal(first, want) {
			t.Fatalf("decoded bytes = %x, want %x", first, want)
		}
	})
}

func FuzzDecodeLZMAEnvelope(f *testing.F) {
	var destination bytes.Buffer
	writer, err := (lzma.WriterConfig{DictCap: lzma.MinDictCap, EOSMarker: true}).NewWriter(&destination)
	if err != nil {
		f.Fatalf("NewWriter() error = %v", err)
	}
	if _, err := writer.Write([]byte{0xa0}); err != nil {
		f.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		f.Fatalf("Close() error = %v", err)
	}
	valid := destination.Bytes()
	f.Add(valid)
	f.Add(valid[:len(valid)-1])
	f.Add(append(append([]byte(nil), valid...), 0))
	f.Add([]byte{0xff, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})

	f.Fuzz(func(t *testing.T, compressed []byte) {
		if len(compressed) > maxCompressedSize {
			return
		}

		first, firstErr := decodeLZMA(compressed)
		second, secondErr := decodeLZMA(compressed)
		firstCode, firstFailed := fuzzDecodeCode(t, firstErr)
		secondCode, secondFailed := fuzzDecodeCode(t, secondErr)

		if firstFailed != secondFailed || firstCode != secondCode || !bytes.Equal(first, second) {
			t.Fatalf("LZMA decode is nondeterministic: codes=(%q,%q), lengths=(%d,%d)", firstCode, secondCode, len(first), len(second))
		}
		if firstFailed {
			if first != nil || second != nil {
				t.Fatalf("failed LZMA decode returned bytes: lengths=(%d,%d)", len(first), len(second))
			}
			return
		}
		if len(first) > maxOutputSize {
			t.Fatalf("accepted output length=%d", len(first))
		}
		if bytes.Equal(compressed, valid) && !bytes.Equal(first, []byte{0xa0}) {
			t.Fatalf("synthetic envelope = %x, want a0", first)
		}
	})
}

func FuzzDecodeMsgpackString(f *testing.F) {
	f.Add([]byte{0xa0, 0})
	f.Add([]byte{0xa0})
	f.Add([]byte{0xd9, 1, 'x'})
	f.Add([]byte{0xda, 0, 1, 'x'})
	f.Add([]byte{0xdb, 0, 0, 0, 1, 'x'})
	f.Add([]byte{0x80})

	f.Fuzz(func(t *testing.T, envelope []byte) {
		if len(envelope) > maxOutputSize {
			return
		}

		first, firstErr := decodeMsgpackString(envelope)
		second, secondErr := decodeMsgpackString(envelope)
		firstCode, firstFailed := fuzzDecodeCode(t, firstErr)
		secondCode, secondFailed := fuzzDecodeCode(t, secondErr)

		if firstFailed != secondFailed || firstCode != secondCode || !bytes.Equal(first, second) {
			t.Fatalf("MessagePack decode is nondeterministic: codes=(%q,%q), lengths=(%d,%d)", firstCode, secondCode, len(first), len(second))
		}
		if firstFailed {
			if first != nil || second != nil {
				t.Fatalf("failed MessagePack decode returned bytes: lengths=(%d,%d)", len(first), len(second))
			}
			return
		}
		headerSize := 1
		var declaredLength uint64
		switch tag := envelope[0]; {
		case tag >= 0xa0 && tag <= 0xbf:
			declaredLength = uint64(tag & 0x1f)
		case tag == 0xd9:
			headerSize = 2
			declaredLength = uint64(envelope[1])
		case tag == 0xda:
			headerSize = 3
			declaredLength = uint64(binary.BigEndian.Uint16(envelope[1:3]))
		case tag == 0xdb:
			headerSize = 5
			declaredLength = uint64(binary.BigEndian.Uint32(envelope[1:5]))
		default:
			t.Fatalf("accepted unsupported MessagePack tag 0x%02x", tag)
		}
		if declaredLength != uint64(len(first)) || !bytes.Equal(envelope[headerSize:], first) {
			t.Fatalf("accepted inexact envelope: declared=%d payload=%d envelope=%x", declaredLength, len(first), envelope)
		}
	})
}

func FuzzParseRoot(f *testing.F) {
	f.Add([]byte(`{"profiles":[]}`))
	f.Add([]byte(`{"inputs":[],"id":"synthetic"}`))
	f.Add([]byte(`{"profiles":[],"inputs":[],"name":"synthetic"}`))
	f.Add([]byte(`[]`))
	f.Add([]byte{0xff})

	f.Fuzz(func(t *testing.T, rawJSON []byte) {
		if len(rawJSON) > maxOutputSize {
			return
		}

		first, firstErr := parseRoot(rawJSON)
		second, secondErr := parseRoot(rawJSON)
		firstCode, firstFailed := fuzzDecodeCode(t, firstErr)
		secondCode, secondFailed := fuzzDecodeCode(t, secondErr)

		if firstFailed != secondFailed || firstCode != secondCode || first != second {
			t.Fatalf("root parse is nondeterministic: first=(%q,%q), second=(%q,%q)", first, firstCode, second, secondCode)
		}
		if firstFailed {
			if first != "" || second != "" {
				t.Fatalf("failed root parse returned kinds (%q,%q)", first, second)
			}
			return
		}
		var root map[string]json.RawMessage
		if err := json.Unmarshal(rawJSON, &root); err != nil {
			t.Fatalf("accepted root cannot be decoded as object: %v", err)
		}
		_, hasProfiles := root["profiles"]
		_, hasInputs := root["inputs"]
		_, hasID := root["id"]
		_, hasName := root["name"]
		switch first {
		case RootBundle:
			if !hasProfiles || hasInputs && (hasID || hasName) {
				t.Fatalf("bundle kind does not match root keys")
			}
		case RootSingle:
			if hasProfiles || !hasInputs || !hasID && !hasName {
				t.Fatalf("single kind does not match root keys")
			}
		default:
			t.Fatalf("accepted unknown root kind %q", first)
		}
	})
}

func fuzzDecodeCode(t testing.TB, err error) (ErrorCode, bool) {
	t.Helper()
	if err == nil {
		return "", false
	}
	var decodeErr *DecodeError
	if !errors.As(err, &decodeErr) {
		t.Fatalf("error type = %T, want *DecodeError", err)
	}
	return decodeErr.Code, true
}
