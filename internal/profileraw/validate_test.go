package profileraw

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestValidateJSON(t *testing.T) {
	t.Parallel()

	depthJSON := func(depth int) json.RawMessage {
		return json.RawMessage(strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth))
	}
	quoted := func(value string) json.RawMessage {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("json.Marshal(%d-byte string): %v", len(value), err)
		}
		return encoded
	}
	escapedString := func(length int) json.RawMessage {
		return json.RawMessage(`"` + strings.Repeat(`\u0061`, length) + `"`)
	}
	objectWithName := func(name string) json.RawMessage {
		encoded, err := json.Marshal(map[string]bool{name: true})
		if err != nil {
			t.Fatalf("json.Marshal(%d-byte member name): %v", len(name), err)
		}
		return encoded
	}

	tests := []struct {
		name     string
		raw      json.RawMessage
		wantCode ErrorCode
	}{
		{name: "depth64", raw: depthJSON(64)},
		{name: "depth65", raw: depthJSON(65), wantCode: ERR_IMPORT_LIMIT_EXCEEDED},
		{name: "string65536", raw: quoted(strings.Repeat("a", 65_536))},
		{name: "string65537", raw: quoted(strings.Repeat("a", 65_537)), wantCode: ERR_IMPORT_LIMIT_EXCEEDED},
		{name: "member_name65536", raw: objectWithName(strings.Repeat("a", 65_536))},
		{name: "member_name65537", raw: objectWithName(strings.Repeat("a", 65_537)), wantCode: ERR_IMPORT_LIMIT_EXCEEDED},
		{name: "multibyte_string65536", raw: quoted(strings.Repeat("界", 21_845) + "a")},
		{name: "multibyte_string65537", raw: quoted(strings.Repeat("界", 21_845) + "ab"), wantCode: ERR_IMPORT_LIMIT_EXCEEDED},
		{name: "escaped_string65536", raw: escapedString(65_536)},
		{name: "escaped_string65537", raw: escapedString(65_537), wantCode: ERR_IMPORT_LIMIT_EXCEEDED},
		{name: "duplicate_root_key", raw: json.RawMessage(`{"a":1,"a":2}`), wantCode: ERR_IMPORT_ROOT},
		{name: "duplicate_escaped_key", raw: json.RawMessage(`{"a":1,"\u0061":2}`), wantCode: ERR_IMPORT_ROOT},
		{name: "duplicate_profile_key", raw: json.RawMessage(`{"profiles":[{"inputs":[],"inputs":[]}]}`), wantCode: ERR_IMPORT_ROOT},
		{name: "duplicate_opaque_macro_key", raw: json.RawMessage(`{"id":1,"inputs":[{"macro":{"step":1,"step":2}}]}`), wantCode: ERR_IMPORT_ROOT},
		{name: "distinct_objects", raw: json.RawMessage(`[{"a":1},{"a":2}]`)},
		{name: "malformed", raw: json.RawMessage(`{"a":`), wantCode: ERR_IMPORT_ROOT},
		{name: "trailing_json", raw: json.RawMessage(`{} []`), wantCode: ERR_IMPORT_ROOT},
		{name: "invalid_utf8", raw: json.RawMessage{'"', 0xff, '"'}, wantCode: ERR_IMPORT_ROOT},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// When
			err := validateJSON(tt.raw)

			// Then
			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("validateJSON() error = %v", err)
				}
				return
			}
			var parseError *ParseError
			if !errors.As(err, &parseError) {
				t.Fatalf("error = %T %v, want *ParseError", err, err)
			}
			if parseError.Code != tt.wantCode || err.Error() != string(tt.wantCode) {
				t.Fatalf("error = %#v (%q), want code-only %q", parseError, err, tt.wantCode)
			}
		})
	}
}
