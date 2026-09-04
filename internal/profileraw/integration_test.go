package profileraw_test

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profiledecode"
	"github.com/sh4869221b/azerlay/internal/profileraw"
	"github.com/ulikunitz/xz/lzma"
)

type goldenScalar struct {
	Kind   profileraw.RawScalarKind `json:"kind"`
	Raw    string                   `json:"raw"`
	Bool   bool                     `json:"bool,omitempty"`
	String string                   `json:"string,omitempty"`
	Number string                   `json:"number,omitempty"`
}

type goldenProfile struct {
	ID      *goldenScalar       `json:"id,omitempty"`
	Name    *goldenScalar       `json:"name,omitempty"`
	Version *goldenScalar       `json:"version,omitempty"`
	Inputs  []map[string]string `json:"inputs"`
	Unknown map[string]string   `json:"unknown"`
}

type goldenBundle struct {
	Version  *goldenScalar     `json:"version,omitempty"`
	Profiles []goldenProfile   `json:"profiles"`
	Unknown  map[string]string `json:"unknown"`
}

type goldenExport struct {
	Bundle *goldenBundle  `json:"bundle,omitempty"`
	Single *goldenProfile `json:"single,omitempty"`
}

type envelopeRecipe struct {
	tag    byte
	codec  *base64.Encoding
	padded bool
}

func TestParseGolden(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind profiledecode.RootKind
	}{
		{name: "bundle", kind: profiledecode.RootBundle},
		{name: "single", kind: profiledecode.RootSingle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Given
			source, expected := readGolden(t, tt.name)
			document := profiledecode.Document{JSON: source, Kind: tt.kind}

			// When
			actual, err := profileraw.Parse(document)

			// Then
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			for index := range source {
				source[index] = 'x'
			}
			assertGolden(t, goldenExportFrom(actual), expected)
		})
	}
}

func TestDecodeThenParse(t *testing.T) {
	t.Parallel()

	// Given
	payload, expected := readGolden(t, "bundle")
	str16 := mustCompressEnvelope(t, messagePackString(t, 0xda, payload))
	str32 := mustCompressEnvelope(t, messagePackString(t, 0xdb, payload))
	urlPadded := mustEncodedEnvelope(t, payload, envelopeRecipe{tag: 0xda, codec: base64.URLEncoding, padded: true})
	urlUnpadded := mustEncodedEnvelope(t, payload, envelopeRecipe{tag: 0xda, codec: base64.RawURLEncoding})
	standardPadded := mustEncodedEnvelope(t, payload, envelopeRecipe{tag: 0xda, codec: base64.StdEncoding, padded: true})
	standardUnpadded := mustEncodedEnvelope(t, payload, envelopeRecipe{tag: 0xda, codec: base64.RawStdEncoding})
	str32Text := base64.RawURLEncoding.EncodeToString(str32)
	tests := []struct {
		name   string
		decode func() (profiledecode.Document, error)
	}{
		{name: "plain JSON", decode: func() (profiledecode.Document, error) { return profiledecode.DecodeText(string(payload)) }},
		{name: "raw LZMA String envelope", decode: func() (profiledecode.Document, error) { return profiledecode.DecodeReader(bytes.NewReader(str16)) }},
		{name: "Base64URL padded String envelope", decode: func() (profiledecode.Document, error) { return profiledecode.DecodeText(urlPadded) }},
		{name: "Base64URL unpadded String envelope", decode: func() (profiledecode.Document, error) { return profiledecode.DecodeText(urlUnpadded) }},
		{name: "standard Base64 padded String envelope", decode: func() (profiledecode.Document, error) { return profiledecode.DecodeText(standardPadded) }},
		{name: "standard Base64 unpadded String envelope", decode: func() (profiledecode.Document, error) { return profiledecode.DecodeText(standardUnpadded) }},
		{name: "str32 String envelope", decode: func() (profiledecode.Document, error) { return profiledecode.DecodeText(str32Text) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			document, decodeErr := tt.decode()
			actual, parseErr := profileraw.Parse(document)

			// Then
			if decodeErr != nil || parseErr != nil {
				t.Fatalf("decode/parse errors = %v / %v", decodeErr, parseErr)
			}
			assertGolden(t, goldenExportFrom(actual), expected)
		})
	}

	t.Run("corrupt envelope remains a decoder error", func(t *testing.T) {
		// Given
		corrupt := base64.RawURLEncoding.EncodeToString(str16[:len(str16)-1])

		// When
		document, err := profiledecode.DecodeText(corrupt)

		// Then
		var decodeError *profiledecode.DecodeError
		if !errors.As(err, &decodeError) || decodeError.Code != profiledecode.ERR_IMPORT_LZMA_CORRUPT {
			t.Fatalf("DecodeText() error = %T %v, want %q", err, err, profiledecode.ERR_IMPORT_LZMA_CORRUPT)
		}
		if !reflect.DeepEqual(document, profiledecode.Document{}) {
			t.Fatalf("Document = %#v, want zero value", document)
		}
	})
}

func assertGolden(t *testing.T, actual goldenExport, expected goldenExport) {
	t.Helper()
	if reflect.DeepEqual(actual, expected) {
		return
	}
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		t.Fatalf("Marshal(actual) error = %v", err)
	}
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("Marshal(expected) error = %v", err)
	}
	t.Fatalf("raw export = %s, want %s", actualJSON, expectedJSON)
}

func readGolden(t *testing.T, name string) (json.RawMessage, goldenExport) {
	t.Helper()
	input, err := os.ReadFile("testdata/" + name + ".input.json")
	if err != nil {
		t.Fatalf("ReadFile(input) error = %v", err)
	}
	expectedJSON, err := os.ReadFile("testdata/" + name + ".expected.json")
	if err != nil {
		t.Fatalf("ReadFile(expected) error = %v", err)
	}
	var expected goldenExport
	if err := json.Unmarshal(expectedJSON, &expected); err != nil {
		t.Fatalf("Unmarshal(expected) error = %v", err)
	}
	return input, expected
}

func goldenExportFrom(export profileraw.RawExport) goldenExport {
	var golden goldenExport
	if export.Bundle != nil {
		profiles := make([]goldenProfile, len(export.Bundle.Profiles))
		for index := range export.Bundle.Profiles {
			profiles[index] = goldenProfileFrom(export.Bundle.Profiles[index])
		}
		golden.Bundle = &goldenBundle{
			Version:  goldenScalarFrom(export.Bundle.Version),
			Profiles: profiles,
			Unknown:  goldenRawMapFrom(export.Bundle.Unknown),
		}
	}
	if export.Single != nil {
		single := goldenProfileFrom(*export.Single)
		golden.Single = &single
	}
	return golden
}

func goldenProfileFrom(profile profileraw.RawProfile) goldenProfile {
	inputs := make([]map[string]string, len(profile.Inputs))
	for index := range profile.Inputs {
		inputs[index] = goldenRawMapFrom(profile.Inputs[index])
	}
	return goldenProfile{
		ID:      goldenScalarFrom(profile.ID),
		Name:    goldenScalarFrom(profile.Name),
		Version: goldenScalarFrom(profile.Version),
		Inputs:  inputs,
		Unknown: goldenRawMapFrom(profile.Unknown),
	}
}

func goldenScalarFrom(scalar *profileraw.RawScalar) *goldenScalar {
	if scalar == nil {
		return nil
	}
	return &goldenScalar{
		Kind:   scalar.Kind,
		Raw:    string(scalar.Raw),
		Bool:   scalar.Bool,
		String: scalar.String,
		Number: string(scalar.Number),
	}
}

func goldenRawMapFrom(values map[string]json.RawMessage) map[string]string {
	golden := make(map[string]string, len(values))
	for key, value := range values {
		golden[key] = string(value)
	}
	return golden
}

func mustEncodedEnvelope(t *testing.T, payload []byte, recipe envelopeRecipe) string {
	t.Helper()
	exclusiveAlphabet := "-_"
	if recipe.codec == base64.StdEncoding || recipe.codec == base64.RawStdEncoding {
		exclusiveAlphabet = "+/"
	}
	for paddingLength := 0; paddingLength < 1024; paddingLength++ {
		candidate := append(append([]byte(nil), payload...), strings.Repeat(" ", paddingLength)...)
		compressed := mustCompressEnvelope(t, messagePackString(t, recipe.tag, candidate))
		encoded := recipe.codec.EncodeToString(compressed)
		if strings.HasSuffix(encoded, "=") == recipe.padded && strings.ContainsAny(encoded, exclusiveAlphabet) {
			return encoded
		}
	}
	t.Fatalf("could not synthesize padded=%t envelope", recipe.padded)
	return ""
}

func mustCompressEnvelope(t *testing.T, envelope []byte) []byte {
	t.Helper()
	var destination bytes.Buffer
	writer, err := (lzma.WriterConfig{DictCap: lzma.MinDictCap, EOSMarker: true}).NewWriter(&destination)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	if _, err := writer.Write(envelope); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return destination.Bytes()
}

func messagePackString(t *testing.T, tag byte, payload []byte) []byte {
	t.Helper()
	if len(payload) > 1<<16-1 {
		t.Fatalf("synthetic payload length = %d", len(payload))
	}
	if tag == 0xda {
		header := []byte{tag, 0, 0}
		binary.BigEndian.PutUint16(header[1:], uint16(len(payload)))
		return append(header, payload...)
	}
	header := []byte{tag, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
	return append(header, payload...)
}
