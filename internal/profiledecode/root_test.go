package profiledecode

import "testing"

func TestParseRoot_WhenSupportedShape_ReturnsKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		json string
		kind RootKind
	}{
		{name: "bundle", json: `{"profiles":[]}`, kind: RootBundle},
		{name: "bundle with optional version", json: `{"version":7,"profiles":[]}`, kind: RootBundle},
		{name: "single by id", json: `{"id":"synthetic","inputs":[]}`, kind: RootSingle},
		{name: "single by name", json: `{"name":"synthetic","inputs":[]}`, kind: RootSingle},
		{name: "single by id and name", json: `{"id":"synthetic","name":"synthetic","inputs":[]}`, kind: RootSingle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			document, err := DecodeText(tt.json)

			if err != nil {
				t.Fatalf("DecodeText() error = %v", err)
			}
			if document.Kind != tt.kind {
				t.Fatalf("Kind = %q, want %q", document.Kind, tt.kind)
			}
		})
	}
}

func TestParseRoot_WhenUnsupportedShape_ReturnsRootError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		json string
	}{
		{name: "both bundle and single", json: `{"profiles":[],"inputs":[],"id":"synthetic"}`},
		{name: "inputs without identity", json: `{"inputs":[]}`},
		{name: "identity without inputs", json: `{"id":"synthetic"}`},
		{name: "unknown object", json: `{"synthetic":true}`},
		{name: "empty object", json: `{}`},
		{name: "array", json: `[]`},
		{name: "string", json: `"synthetic"`},
		{name: "number", json: `7`},
		{name: "boolean", json: `true`},
		{name: "null", json: `null`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			document, err := decodeSyntheticEnvelope(t, msgpackString(t, 0xd9, []byte(tt.json)))

			assertDecodeError(t, err, ERR_IMPORT_ROOT)
			assertZeroDocument(t, document)
		})
	}
}

func TestDecodeText_WhenJSONMalformedOrTrailing_ReturnsJSONError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		json string
	}{
		{name: "malformed object", json: `{"profiles":`},
		{name: "second object", json: `{"profiles":[]} {"profiles":[]}`},
		{name: "trailing scalar", json: `{"profiles":[]} true`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			document, err := DecodeText(tt.json)

			assertDecodeError(t, err, ERR_IMPORT_JSON)
			assertZeroDocument(t, document)
		})
	}
}
