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

type jsonContext uint8

const (
	opaqueJSON jsonContext = iota
	exportJSON
	rootJSON
	bundleJSON
	profileJSON
	profilesJSON
	inputsJSON
)

func validateJSON(raw json.RawMessage, context jsonContext) error {
	if !utf8.Valid(raw) {
		return &ParseError{Code: ERR_IMPORT_ROOT}
	}
	if context == exportJSON {
		context = preflightRootContext(raw)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return &ParseError{Code: ERR_IMPORT_ROOT}
	}
	if err := validateJSONToken(decoder, token, 0, context); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return &ParseError{Code: ERR_IMPORT_ROOT}
	}
	return nil
}

func validateJSONToken(decoder *json.Decoder, token json.Token, depth int, context jsonContext) error {
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
			if err := validateJSONToken(decoder, valueToken, containerDepth, fieldContext(context, key)); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return &ParseError{Code: ERR_IMPORT_ROOT}
		}
	case '[':
		limit, memberContext := 0, opaqueJSON
		switch context {
		case profilesJSON:
			limit, memberContext = maxProfiles, profileJSON
		case inputsJSON:
			limit = maxInputs
		}
		count := 0
		for decoder.More() {
			// Reject before decoding or retaining the next member. Array-like
			// values inside unknown fields and inputs remain opaque here.
			if limit != 0 && count == limit {
				return &ParseError{Code: ERR_IMPORT_LIMIT_EXCEEDED}
			}
			valueToken, err := decoder.Token()
			if err != nil {
				return &ParseError{Code: ERR_IMPORT_ROOT}
			}
			if err := validateJSONToken(decoder, valueToken, containerDepth, memberContext); err != nil {
				return err
			}
			count++
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

func fieldContext(context jsonContext, key string) jsonContext {
	switch context {
	case rootJSON:
		if key == "profiles" {
			return profilesJSON
		}
		if key == "inputs" {
			return inputsJSON
		}
	case bundleJSON:
		if key == "profiles" {
			return profilesJSON
		}
	case profileJSON:
		if key == "inputs" {
			return inputsJSON
		}
	}
	return opaqueJSON
}
