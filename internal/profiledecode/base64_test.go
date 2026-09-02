package profiledecode

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func TestDetectOuter_WhenEachApprovedForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		form outerForm
	}{
		{name: "raw JSON object", text: `{"profiles":[]}`, form: outerRawJSON},
		{name: "raw JSON array", text: `[]`, form: outerRawJSON},
		{name: "padded Base64URL", text: "-_8=", form: outerBase64URLPadded},
		{name: "unpadded Base64URL", text: "-_8", form: outerBase64URLUnpadded},
		{name: "padded standard Base64", text: "+/8=", form: outerBase64StandardPadded},
		{name: "unpadded standard Base64", text: "+/8", form: outerBase64StandardUnpadded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			form, err := detectOuter(tt.text)

			if err != nil {
				t.Fatalf("detectOuter() error = %v", err)
			}
			if form != tt.form {
				t.Fatalf("detectOuter() = %v, want %v", form, tt.form)
			}
		})
	}
}

func TestDetectOuter_WhenBase64IsAlphanumericOnly_SelectsURLDeterministically(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		form outerForm
	}{
		{name: "padded", text: "YWI=", form: outerBase64URLPadded},
		{name: "unpadded", text: "YWJj", form: outerBase64URLUnpadded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			form, err := detectOuter(tt.text)

			if err != nil {
				t.Fatalf("detectOuter() error = %v", err)
			}
			if form != tt.form {
				t.Fatalf("detectOuter() = %v, want %v", form, tt.form)
			}
		})
	}
}

func TestDecodeBase64_WhenEachVariant_DecodesExactBytes(t *testing.T) {
	t.Parallel()

	want := []byte{0xfb, 0xff}
	tests := []struct {
		name string
		text string
	}{
		{name: "padded Base64URL", text: "-_8="},
		{name: "unpadded Base64URL", text: "-_8"},
		{name: "padded standard Base64", text: "+/8="},
		{name: "unpadded standard Base64", text: "+/8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			form, err := detectOuter(tt.text)
			if err != nil {
				t.Fatalf("detectOuter() error = %v", err)
			}
			got, err := decodeBase64(tt.text, form)

			if err != nil {
				t.Fatalf("decodeBase64() error = %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("decodeBase64() = %x, want %x", got, want)
			}
		})
	}
}

func TestDecodeBase64_WhenInvalid_RejectsWithStableCodeAndNilBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
	}{
		{name: "mixed alphabets", text: "+_8="},
		{name: "nonterminal padding", text: "YW=Jj"},
		{name: "excess padding", text: "YQ==="},
		{name: "embedded space", text: "YW Jj"},
		{name: "embedded newline", text: "YW\nJj"},
		{name: "invalid character", text: "YW$J"},
		{name: "invalid Unicode", text: "YWéJ"},
		{name: "modulo one length", text: "A"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			form, detectErr := detectOuter(tt.text)
			var got []byte
			decodeErr := detectErr
			if decodeErr == nil {
				got, decodeErr = decodeBase64(tt.text, form)
			}

			assertDecodeError(t, decodeErr, ERR_IMPORT_ENCODING)
			if got != nil {
				t.Fatalf("decoded bytes = %x, want nil", got)
			}
			if strings.Contains(decodeErr.Error(), tt.text) && tt.text != "" {
				t.Fatalf("error disclosed source: %q", decodeErr)
			}
		})
	}
}

func TestDecodeBase64_WhenEmpty_RejectsEveryBase64Form(t *testing.T) {
	t.Parallel()

	forms := []outerForm{
		outerBase64URLPadded,
		outerBase64URLUnpadded,
		outerBase64StandardPadded,
		outerBase64StandardUnpadded,
	}
	for _, form := range forms {
		got, err := decodeBase64("", form)

		assertDecodeError(t, err, ERR_IMPORT_ENCODING)
		if got != nil {
			t.Fatalf("decodeBase64() = %x, want nil", got)
		}
	}
}

func TestDecodeBase64_WhenLimitBoundary_AcceptsExactLimit(t *testing.T) {
	t.Parallel()

	want := bytes.Repeat([]byte{0}, maxCompressedSize)
	text := base64.RawURLEncoding.EncodeToString(want)
	form, err := detectOuter(text)
	if err != nil {
		t.Fatalf("detectOuter() error = %v", err)
	}

	got, err := decodeBase64(text, form)

	if err != nil {
		t.Fatalf("decodeBase64() error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("decoded bytes differ at exact %d-byte limit", maxCompressedSize)
	}
}

func TestDecodeBase64_WhenLimitBoundary_RejectsLimitPlusOne(t *testing.T) {
	t.Parallel()

	text := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0}, maxCompressedSize+1))
	form, err := detectOuter(text)
	if err != nil {
		t.Fatalf("detectOuter() error = %v", err)
	}

	got, err := decodeBase64(text, form)

	assertDecodeError(t, err, ERR_IMPORT_LIMIT_EXCEEDED)
	if got != nil {
		t.Fatalf("decodeBase64() = %x, want nil", got)
	}
}

func TestDecodeBase64_WhenValidText_ReachesLZMAHeaderStage(t *testing.T) {
	t.Parallel()

	for _, text := range []string{"-_8=", "-_8", "+/8=", "+/8"} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()

			document, err := DecodeText(text)

			assertDecodeError(t, err, ERR_IMPORT_LZMA_HEADER)
			assertZeroDocument(t, document)
		})
	}
}
