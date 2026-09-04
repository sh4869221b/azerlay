package profileraw

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

const (
	maxJSONContainerDepth = 64
	maxJSONStringBytes    = 65_536
)

func validateJSON(raw json.RawMessage) error {
	if !utf8.Valid(raw) {
		return &ParseError{Code: ERR_IMPORT_ROOT}
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return &ParseError{Code: ERR_IMPORT_ROOT}
	}
	if err := validateJSONToken(decoder, token, 0); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return &ParseError{Code: ERR_IMPORT_ROOT}
	}
	return nil
}

func validateJSONToken(decoder *json.Decoder, token json.Token, depth int) error {
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		if value, isString := token.(string); isString && len(value) > maxJSONStringBytes {
			return &ParseError{Code: ERR_IMPORT_LIMIT_EXCEEDED}
		}
		return nil
	}

	containerDepth := depth + 1
	if containerDepth > maxJSONContainerDepth {
		return &ParseError{Code: ERR_IMPORT_LIMIT_EXCEEDED}
	}

	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return &ParseError{Code: ERR_IMPORT_ROOT}
			}
			key, ok := keyToken.(string)
			if !ok {
				return &ParseError{Code: ERR_IMPORT_ROOT}
			}
			if len(key) > maxJSONStringBytes {
				return &ParseError{Code: ERR_IMPORT_LIMIT_EXCEEDED}
			}
			if _, duplicate := seen[key]; duplicate {
				return &ParseError{Code: ERR_IMPORT_ROOT}
			}
			seen[key] = struct{}{}

			valueToken, err := decoder.Token()
			if err != nil {
				return &ParseError{Code: ERR_IMPORT_ROOT}
			}
			if err := validateJSONToken(decoder, valueToken, containerDepth); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return &ParseError{Code: ERR_IMPORT_ROOT}
		}
	case '[':
		for decoder.More() {
			valueToken, err := decoder.Token()
			if err != nil {
				return &ParseError{Code: ERR_IMPORT_ROOT}
			}
			if err := validateJSONToken(decoder, valueToken, containerDepth); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return &ParseError{Code: ERR_IMPORT_ROOT}
		}
	default:
		return &ParseError{Code: ERR_IMPORT_ROOT}
	}
	return nil
}
