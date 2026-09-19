package control

import (
	"encoding/json"
	"io"
)

func ReadResponse(reader io.Reader, request Request) (Response, error) {
	line, err := readLine(reader)
	if err != nil {
		return Response{}, err
	}
	fields, err := object(line, "version", "id", "ok", "result", "error")
	if err != nil {
		return Response{}, err
	}
	response := Response{}
	if null(fields["version"]) || json.Unmarshal(fields["version"], &response.Version) != nil {
		return Response{}, NewError(ERR_CONTROL_REQUEST)
	}
	if response.Version != ProtocolVersion {
		return Response{}, NewError(ERR_CONTROL_VERSION)
	}
	if json.Unmarshal(fields["id"], &response.ID) != nil || response.ID == nil || *response.ID != request.ID || null(fields["ok"]) || json.Unmarshal(fields["ok"], &response.OK) != nil {
		return Response{}, NewError(ERR_CONTROL_REQUEST)
	}
	if response.OK {
		if !null(fields["error"]) {
			return Response{}, NewError(ERR_CONTROL_REQUEST)
		}
		response.Result, err = decodeResult(fields["result"], request.Method)
	} else {
		if !null(fields["result"]) {
			return Response{}, NewError(ERR_CONTROL_REQUEST)
		}
		response.Error, err = decodeError(fields["error"])
	}
	if err != nil {
		return Response{}, err
	}
	return response, nil
}

func decodeError(raw []byte) (*Error, error) {
	var value Error
	if _, err := decodeObject(raw, &value, "code", "stage", "summary", "remediation"); err != nil {
		return nil, err
	}
	// Never present untrusted diagnostic text as a safe remote error.
	expected := NewError(value.Code)
	if value != *expected {
		return nil, NewError(ERR_CONTROL_REQUEST)
	}
	return expected, nil
}

func decodeResult(raw []byte, method Method) (Result, error) {
	switch method {
	case MethodShow, MethodHide, MethodToggle:
		var result VisibilityResult
		_, err := decodeObject(raw, &result, "visible")
		return result, err
	case MethodReload:
		var result ReloadResult
		_, err := decodeObject(raw, &result, "accepted", "request_generation")
		if err != nil {
			return nil, err
		}
		if !result.Accepted {
			return nil, NewError(ERR_CONTROL_REQUEST)
		}
		return result, nil
	case MethodQuit:
		var result QuitResult
		_, err := decodeObject(raw, &result, "quitting")
		if err != nil {
			return nil, err
		}
		if !result.Quitting {
			return nil, NewError(ERR_CONTROL_REQUEST)
		}
		return result, nil
	case MethodSelect:
		var result SelectionResult
		fields, err := decodeObject(raw, &result, "active_profile", "generation")
		if err != nil {
			return nil, err
		}
		if err := validateProfile(fields["active_profile"]); err != nil {
			return nil, err
		}
		return result, nil
	case MethodStatus:
		return decodeStatus(raw)
	default:
		return nil, NewError(ERR_CONTROL_METHOD)
	}
}

// decodeObject is confined to the wire boundary. Nullable fields use a trailing
// question mark in this private field list; the JSON field name has no suffix.
func decodeObject(raw []byte, target any, names ...string) (map[string]json.RawMessage, error) {
	keys := make([]string, len(names))
	for i, name := range names {
		keys[i] = name
		if name[len(name)-1] == '?' {
			keys[i] = name[:len(name)-1]
		}
	}
	fields, err := object(raw, keys...)
	if err != nil {
		return nil, err
	}
	for i, name := range names {
		if name[len(name)-1] != '?' && null(fields[keys[i]]) {
			return nil, NewError(ERR_CONTROL_REQUEST)
		}
	}
	if json.Unmarshal(raw, target) != nil {
		return nil, NewError(ERR_CONTROL_REQUEST)
	}
	return fields, nil
}

func validateProfile(raw []byte) error {
	var profile ProfileStatus
	_, err := decodeObject(raw, &profile, "source", "source_ref", "profile_index", "name?")
	if err != nil {
		return err
	}
	if profile.Source != "imported" || profile.SourceRef == "" || profile.ProfileIndex < 1 {
		return NewError(ERR_CONTROL_REQUEST)
	}
	return nil
}

func validateDiagnostic(raw []byte) error {
	var diagnostic Diagnostic
	_, err := decodeObject(raw, &diagnostic, "code", "stage", "reason")
	if err != nil {
		return err
	}
	if diagnostic.Code == "" || diagnostic.Stage == "" || diagnostic.Reason == "" {
		return NewError(ERR_CONTROL_REQUEST)
	}
	return nil
}

func decodeStatus(raw []byte) (Status, error) {
	var status Status
	fields, err := decodeObject(raw, &status,
		"schema_version", "uptime_seconds", "visible", "active_profile?", "device?", "event_nodes", "event_rate?", "dropped_count?", "resync_count?", "render_rate?", "last_reload", "generation", "degraded_reasons")
	if err != nil {
		return Status{}, err
	}
	if status.SchemaVersion != 1 || status.UptimeSeconds < 0 || status.Generation == 0 {
		return Status{}, NewError(ERR_CONTROL_REQUEST)
	}
	if !null(fields["active_profile"]) {
		if err := validateProfile(fields["active_profile"]); err != nil {
			return Status{}, err
		}
	}
	var reload ReloadStatus
	reloadFields, err := decodeObject(fields["last_reload"], &reload, "request_generation", "config_generation", "config_failure?", "watch_failure?")
	if err != nil {
		return Status{}, err
	}
	for _, name := range []string{"config_failure", "watch_failure"} {
		if !null(reloadFields[name]) {
			if err := validateDiagnostic(reloadFields[name]); err != nil {
				return Status{}, err
			}
		}
	}
	var diagnostics []json.RawMessage
	if json.Unmarshal(fields["degraded_reasons"], &diagnostics) != nil {
		return Status{}, NewError(ERR_CONTROL_REQUEST)
	}
	for _, diagnostic := range diagnostics {
		if err := validateDiagnostic(diagnostic); err != nil {
			return Status{}, err
		}
	}
	return status, nil
}
