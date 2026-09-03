package profiledecode

import (
	"encoding/json"
	"unicode/utf8"
)

func decodeJSONDocument(rawJSON []byte) (Document, error) {
	kind, err := parseRoot(rawJSON)
	if err != nil {
		return Document{}, err
	}
	return Document{JSON: append(json.RawMessage(nil), rawJSON...), Kind: kind}, nil
}

func parseRoot(rawJSON []byte) (RootKind, error) {
	if !utf8.Valid(rawJSON) {
		return "", newDecodeError(ERR_IMPORT_UTF8, nil)
	}
	if !json.Valid(rawJSON) {
		return "", newDecodeError(ERR_IMPORT_JSON, nil)
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(rawJSON, &root); err != nil {
		return "", newDecodeError(ERR_IMPORT_ROOT, nil)
	}
	_, hasProfiles := root["profiles"]
	_, hasInputs := root["inputs"]
	_, hasID := root["id"]
	_, hasName := root["name"]
	isBundle := hasProfiles
	isSingle := hasInputs && (hasID || hasName)
	if isBundle == isSingle {
		return "", newDecodeError(ERR_IMPORT_ROOT, nil)
	}
	if isSingle {
		return RootSingle, nil
	}
	return RootBundle, nil
}
