package profiledecode

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ulikunitz/xz/lzma"
)

func TestDecodeEntrypoints_WhenBinaryLZMAPropertyLooksLikeJSONPrefix_ReturnsExactBundle(t *testing.T) {
	t.Parallel()

	// Given
	bundle := []byte(`{"profiles":[]}`)
	properties := []struct {
		name string
		code byte
	}{
		{name: "left bracket", code: 0x5b},
		{name: "left brace", code: 0x7b},
	}
	for _, property := range properties {
		t.Run(property.name, func(t *testing.T) {
			lzmaProperties, err := lzma.PropertiesForCode(property.code)
			if err != nil {
				t.Fatalf("PropertiesForCode(0x%x) error = %v", property.code, err)
			}
			var compressed bytes.Buffer
			writer, err := (lzma.WriterConfig{
				Properties: &lzmaProperties,
				DictCap:    lzma.MinDictCap,
				EOSMarker:  true,
			}).NewWriter(&compressed)
			if err != nil {
				t.Fatalf("NewWriter() error = %v", err)
			}
			if _, err := writer.Write(msgpackString(t, 0xa0, bundle)); err != nil {
				t.Fatalf("Write() error = %v", err)
			}
			if err := writer.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
			source := compressed.Bytes()
			if source[0] != property.code {
				t.Fatalf("property byte = 0x%x, want 0x%x", source[0], property.code)
			}
			path := filepath.Join(t.TempDir(), "profile.lzma")
			if err := os.WriteFile(path, source, 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			entrypoints := []struct {
				name   string
				decode func() (Document, error)
			}{
				{name: "raw reader", decode: func() (Document, error) { return DecodeReader(bytes.NewReader(source)) }},
				{name: "raw file", decode: func() (Document, error) { return DecodeFile(path) }},
				{name: "Base64URL text", decode: func() (Document, error) {
					return DecodeText(base64.RawURLEncoding.EncodeToString(source))
				}},
			}
			for _, entrypoint := range entrypoints {
				t.Run(entrypoint.name, func(t *testing.T) {
					// When
					document, err := entrypoint.decode()

					// Then
					if err != nil {
						t.Fatalf("decode() error = %v", err)
					}
					if !bytes.Equal(document.JSON, bundle) {
						t.Fatalf("JSON = %q, want %q", document.JSON, bundle)
					}
					if document.Kind != RootBundle {
						t.Fatalf("kind = %q, want %q", document.Kind, RootBundle)
					}
				})
			}
		})
	}
}

func TestDecodeEntrypoints_WhenLongMalformedBase64_ReturnEncodingAndZeroDocument(t *testing.T) {
	t.Parallel()

	// Given
	textBlock := strings.Repeat("A", 64)
	tests := []struct {
		name   string
		source string
	}{
		{name: "embedded whitespace", source: textBlock + " " + textBlock},
		{name: "invalid character", source: textBlock + "$" + textBlock},
		{name: "mixed URL and standard alphabets", source: textBlock + "+-" + textBlock},
		{name: "nonterminal padding", source: textBlock + "=A" + textBlock},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "malformed.txt")
			if err := os.WriteFile(path, []byte(tt.source), 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			entrypoints := []struct {
				name   string
				decode func() (Document, error)
			}{
				{name: "text", decode: func() (Document, error) { return DecodeText(tt.source) }},
				{name: "reader", decode: func() (Document, error) { return DecodeReader(strings.NewReader(tt.source)) }},
				{name: "file", decode: func() (Document, error) { return DecodeFile(path) }},
			}
			for _, entrypoint := range entrypoints {
				t.Run(entrypoint.name, func(t *testing.T) {
					// When
					document, err := entrypoint.decode()

					// Then
					assertDecodeError(t, err, ERR_IMPORT_ENCODING)
					assertZeroDocument(t, document)
				})
			}
		})
	}
}
