package profiledecode

import (
	"encoding/json"
	"errors"
	"io"
	"os"
)

const (
	maxSourceSize     = 16 << 20
	maxCompressedSize = 8 << 20
	maxDictionarySize = 64 << 20
	maxOutputSize     = 64 << 20
)

// RootKind is the provisional decoded JSON root shape.
type RootKind string

const (
	RootBundle RootKind = "bundle"
	RootSingle RootKind = "single"
)

// Document is an owned JSON export and its provisional root shape.
type Document struct {
	JSON json.RawMessage
	Kind RootKind
}

// DecodeText decodes a normalized Raw JSON export.
func DecodeText(text string) (Document, error) {
	if len(text) > maxSourceSize {
		return Document{}, newDecodeError(ERR_IMPORT_LIMIT_EXCEEDED, nil)
	}

	normalized, err := normalizeOuterText(text)
	if err != nil {
		return Document{}, newDecodeError(ERR_IMPORT_ENCODING, err)
	}
	form, err := detectOuter(normalized)
	if err != nil {
		return Document{}, err
	}
	switch form {
	case outerRawJSON:
	case outerBase64URLPadded, outerBase64URLUnpadded, outerBase64StandardPadded, outerBase64StandardUnpadded:
		if _, err := decodeBase64(normalized, form); err != nil {
			return Document{}, err
		}
		return Document{}, newDecodeError(ERR_IMPORT_LZMA_HEADER, nil)
	default:
		return Document{}, newDecodeError(ERR_IMPORT_ENCODING, nil)
	}

	rawJSON := []byte(normalized)
	if !json.Valid(rawJSON) {
		return Document{}, newDecodeError(ERR_IMPORT_JSON, nil)
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(rawJSON, &root); err != nil {
		return Document{}, newDecodeError(ERR_IMPORT_ROOT, nil)
	}
	_, hasProfiles := root["profiles"]
	_, hasInputs := root["inputs"]
	_, hasID := root["id"]
	_, hasName := root["name"]
	isBundle := hasProfiles
	isSingle := hasInputs && (hasID || hasName)
	if isBundle == isSingle {
		return Document{}, newDecodeError(ERR_IMPORT_ROOT, nil)
	}

	kind := RootBundle
	if isSingle {
		kind = RootSingle
	}
	return Document{JSON: append(json.RawMessage(nil), rawJSON...), Kind: kind}, nil
}

// DecodeReader reads one bounded source and delegates to DecodeText.
func DecodeReader(reader io.Reader) (Document, error) {
	if reader == nil {
		return Document{}, newDecodeError(ERR_IMPORT_ENCODING, errors.New("nil reader"))
	}

	source, err := io.ReadAll(io.LimitReader(reader, int64(maxSourceSize)+1))
	if err != nil {
		return Document{}, newDecodeError(ERR_IMPORT_ENCODING, err)
	}
	if len(source) > maxSourceSize {
		return Document{}, newDecodeError(ERR_IMPORT_LIMIT_EXCEEDED, nil)
	}
	return DecodeText(string(source))
}

// DecodeFile opens a source read-only and delegates to DecodeReader.
func DecodeFile(path string) (document Document, err error) {
	file, openErr := os.Open(path)
	if openErr != nil {
		return Document{}, newDecodeError(ERR_IMPORT_ENCODING, openErr)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			document = Document{}
			err = newDecodeError(ERR_IMPORT_ENCODING, closeErr)
		}
	}()

	return DecodeReader(file)
}
