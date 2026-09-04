package profileraw

import (
	"bytes"
	"encoding/json"

	"github.com/sh4869221b/azerlay/internal/profiledecode"
)

const (
	maxProfiles = 512
	maxInputs   = 256
)

// Parse constructs an authoritative raw model from a decoded document.
func Parse(document profiledecode.Document) (RawExport, error) {
	if err := validateJSON(document.JSON); err != nil {
		return RawExport{}, err
	}

	token := bytes.TrimSpace(document.JSON)
	if len(token) == 0 || token[0] != '{' {
		return RawExport{}, &ParseError{Code: ERR_IMPORT_ROOT}
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(token, &root); err != nil {
		return RawExport{}, &ParseError{Code: ERR_IMPORT_ROOT}
	}

	_, hasProfiles := root["profiles"]
	_, hasInputs := root["inputs"]
	_, hasID := root["id"]
	_, hasName := root["name"]
	isBundle := hasProfiles
	isSingle := hasInputs && (hasID || hasName)
	if isBundle == isSingle {
		return RawExport{}, &ParseError{Code: ERR_IMPORT_ROOT}
	}

	if isBundle {
		if document.Kind != profiledecode.RootBundle {
			return RawExport{}, &ParseError{Code: ERR_IMPORT_ROOT}
		}
		bundle, err := parseBundle(root)
		if err != nil {
			return RawExport{}, err
		}
		return RawExport{Bundle: &bundle}, nil
	}
	if document.Kind != profiledecode.RootSingle {
		return RawExport{}, &ParseError{Code: ERR_IMPORT_ROOT}
	}
	profile, err := parseProfile(root)
	if err != nil {
		return RawExport{}, err
	}
	return RawExport{Single: &profile}, nil
}

func parseBundle(fields map[string]json.RawMessage) (RawBundle, error) {
	version, err := parseVersion(fields["version"])
	if err != nil {
		return RawBundle{}, err
	}

	profilesToken := bytes.TrimSpace(fields["profiles"])
	if len(profilesToken) == 0 || profilesToken[0] != '[' {
		return RawBundle{}, &ParseError{Code: ERR_IMPORT_ROOT}
	}
	var rawProfiles []json.RawMessage
	if err := json.Unmarshal(profilesToken, &rawProfiles); err != nil {
		return RawBundle{}, &ParseError{Code: ERR_IMPORT_ROOT}
	}
	if len(rawProfiles) > maxProfiles {
		return RawBundle{}, &ParseError{Code: ERR_IMPORT_LIMIT_EXCEEDED}
	}

	profiles := make([]RawProfile, len(rawProfiles))
	for index, rawProfile := range rawProfiles {
		profileToken := bytes.TrimSpace(rawProfile)
		if len(profileToken) == 0 || profileToken[0] != '{' {
			return RawBundle{}, &ParseError{Code: ERR_IMPORT_ROOT}
		}
		var profileFields map[string]json.RawMessage
		if err := json.Unmarshal(profileToken, &profileFields); err != nil {
			return RawBundle{}, &ParseError{Code: ERR_IMPORT_ROOT}
		}
		profile, err := parseProfile(profileFields)
		if err != nil {
			return RawBundle{}, err
		}
		profiles[index] = profile
	}

	unknown := make(map[string]json.RawMessage)
	for key, value := range fields {
		if key != "version" && key != "profiles" {
			unknown[key] = value
		}
	}
	return RawBundle{Version: version, Profiles: profiles, Unknown: unknown}, nil
}

func parseProfile(fields map[string]json.RawMessage) (RawProfile, error) {
	version, err := parseVersion(fields["version"])
	if err != nil {
		return RawProfile{}, err
	}
	id, err := parseScalar(fields["id"])
	if err != nil {
		return RawProfile{}, err
	}
	name, err := parseScalar(fields["name"])
	if err != nil {
		return RawProfile{}, err
	}

	inputsToken := bytes.TrimSpace(fields["inputs"])
	if len(inputsToken) == 0 || inputsToken[0] != '[' {
		return RawProfile{}, &ParseError{Code: ERR_IMPORT_ROOT}
	}
	var rawInputs []json.RawMessage
	if err := json.Unmarshal(inputsToken, &rawInputs); err != nil {
		return RawProfile{}, &ParseError{Code: ERR_IMPORT_ROOT}
	}
	if len(rawInputs) > maxInputs {
		return RawProfile{}, &ParseError{Code: ERR_IMPORT_LIMIT_EXCEEDED}
	}

	inputs := make([]RawInput, len(rawInputs))
	for index, rawInput := range rawInputs {
		inputToken := bytes.TrimSpace(rawInput)
		if len(inputToken) == 0 || inputToken[0] != '{' {
			return RawProfile{}, &ParseError{Code: ERR_IMPORT_ROOT}
		}
		var input RawInput
		if err := json.Unmarshal(inputToken, &input); err != nil {
			return RawProfile{}, &ParseError{Code: ERR_IMPORT_ROOT}
		}
		inputs[index] = input
	}

	unknown := make(map[string]json.RawMessage)
	for key, value := range fields {
		if key != "id" && key != "name" && key != "version" && key != "inputs" {
			unknown[key] = value
		}
	}
	return RawProfile{ID: id, Name: name, Version: version, Inputs: inputs, Unknown: unknown}, nil
}

func parseVersion(raw json.RawMessage) (*RawScalar, error) {
	if raw == nil {
		return nil, nil
	}
	token := bytes.TrimSpace(raw)
	if len(token) > 0 && (token[0] == '{' || token[0] == '[') {
		return nil, &ParseError{Code: ERR_IMPORT_UNSUPPORTED_VERSION}
	}
	return parseScalar(raw)
}

func parseScalar(raw json.RawMessage) (*RawScalar, error) {
	if raw == nil {
		return nil, nil
	}
	var scalar RawScalar
	if err := json.Unmarshal(raw, &scalar); err != nil {
		return nil, &ParseError{Code: ERR_IMPORT_ROOT}
	}
	return &scalar, nil
}
