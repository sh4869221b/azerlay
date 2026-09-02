package profiledecode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
