package profiledecode

import (
	"encoding/base64"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

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

func encodeLZMATestText(compressed []byte) string {
	return base64.RawURLEncoding.EncodeToString(compressed)
}

func encodeSyntheticEnvelopeText(t *testing.T, envelope []byte) string {
	t.Helper()
	return encodeLZMATestText(mustCompressLZMA(t, envelope, false))
}

type syntheticEncoding struct {
	tag    byte
	codec  *base64.Encoding
	padded bool
}

func mustEncodedEnvelope(t *testing.T, payload []byte, recipe syntheticEncoding) (string, []byte) {
	t.Helper()

	exclusiveAlphabet := "-_"
	if recipe.codec == base64.StdEncoding || recipe.codec == base64.RawStdEncoding {
		exclusiveAlphabet = "+/"
	}
	for paddingLength := 0; paddingLength < 1024; paddingLength++ {
		candidate := append(append([]byte(nil), payload...), strings.Repeat(" ", paddingLength)...)
		compressed := mustCompressLZMA(t, msgpackString(t, recipe.tag, candidate), false)
		encoded := recipe.codec.EncodeToString(compressed)
		if strings.HasSuffix(encoded, "=") != recipe.padded || !strings.ContainsAny(encoded, exclusiveAlphabet) {
			continue
		}
		return encoded, candidate
	}
	t.Fatalf("could not synthesize padded=%t envelope", recipe.padded)
	return "", nil
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
