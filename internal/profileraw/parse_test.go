package profileraw

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profiledecode"
)

func TestParseRoots(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		kind profiledecode.RootKind
	}{
		{name: "canonical bundle", raw: `{"version":"synthetic-1","profiles":[]}`, kind: profiledecode.RootBundle},
		{name: "canonical single", raw: `{"id":null,"inputs":[]}`, kind: profiledecode.RootSingle},
		{name: "bundle profile without identity", raw: `{"profiles":[{"inputs":[]}]}`, kind: profiledecode.RootBundle},
		{name: "bundle null versions", raw: `{"version":null,"profiles":[{"version":null,"inputs":[]}]}`, kind: profiledecode.RootBundle},
		{name: "bundle boolean versions", raw: `{"version":true,"profiles":[{"version":false,"inputs":[]}]}`, kind: profiledecode.RootBundle},
		{name: "single by name", raw: `{"name":"","version":7,"inputs":[]}`, kind: profiledecode.RootSingle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Given
			document := profiledecode.Document{JSON: json.RawMessage(tt.raw), Kind: tt.kind}

			// When
			export, err := Parse(document)

			// Then
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if (export.Bundle == nil) == (export.Single == nil) {
				t.Fatalf("RawExport = %#v, want exactly one root", export)
			}
			if export.Bundle != nil && export.Bundle.Profiles == nil {
				t.Fatal("bundle Profiles = nil, want non-nil empty or populated slice")
			}
			if export.Single != nil && export.Single.Inputs == nil {
				t.Fatal("single Inputs = nil, want non-nil empty slice")
			}
		})
	}
}

func TestParseLimits(t *testing.T) {
	t.Parallel()

	profiles := func(count int) string {
		return `{"profiles":[` + strings.TrimSuffix(strings.Repeat(`{"inputs":[]},`, count), ",") + `]}`
	}
	inputs := func(count int) string {
		return `{"id":1,"inputs":[` + strings.TrimSuffix(strings.Repeat(`{},`, count), ",") + `]}`
	}
	tests := []struct {
		name      string
		raw       string
		kind      profiledecode.RootKind
		wantCount int
		wantCode  ErrorCode
	}{
		{name: "profiles 512", raw: profiles(512), kind: profiledecode.RootBundle, wantCount: 512},
		{name: "profiles 513", raw: profiles(513), kind: profiledecode.RootBundle, wantCode: ERR_IMPORT_LIMIT_EXCEEDED},
		{name: "inputs 256", raw: inputs(256), kind: profiledecode.RootSingle, wantCount: 256},
		{name: "inputs 257", raw: inputs(257), kind: profiledecode.RootSingle, wantCode: ERR_IMPORT_LIMIT_EXCEEDED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			export, err := Parse(profiledecode.Document{JSON: json.RawMessage(tt.raw), Kind: tt.kind})
			if tt.wantCode != "" {
				assertParseFailure(t, export, err, tt.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if export.Bundle != nil && len(export.Bundle.Profiles) != tt.wantCount {
				t.Fatalf("len(Profiles) = %d, want %d", len(export.Bundle.Profiles), tt.wantCount)
			}
			if export.Single != nil && len(export.Single.Inputs) != tt.wantCount {
				t.Fatalf("len(Inputs) = %d, want %d", len(export.Single.Inputs), tt.wantCount)
			}
		})
	}
}

func TestParsePreservation(t *testing.T) {
	t.Parallel()

	// Given
	source := json.RawMessage(`{"version":1e+03,"profiles":[{"id":null,"name":"0\u00301","version":-0,"inputs":[{"macro":{"steps":[1]},"number":9223372036854775808,"flag":true,"nothing":null}],"profileUnknown":[1,2]},{"inputs":[]}],"rootUnknown":{"x":true}}`)
	document := profiledecode.Document{JSON: source, Kind: profiledecode.RootBundle}

	// When
	export, err := Parse(document)

	// Then
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	for index := range source {
		source[index] = 'x'
	}
	bundle := export.Bundle
	if bundle == nil || export.Single != nil || bundle.Version == nil || string(bundle.Version.Raw) != "1e+03" || bundle.Version.Number != json.Number("1e+03") {
		t.Fatalf("RawExport root/version = %#v, want owned bundle number 1e+03", export)
	}
	profile := bundle.Profiles[0]
	if profile.ID == nil || profile.ID.Kind != ScalarNull || profile.Name == nil || profile.Name.String != "001" || string(profile.Name.Raw) != `"0\u00301"` || profile.Version == nil || string(profile.Version.Raw) != "-0" {
		t.Fatalf("profile scalars = %#v, want literal null/string/number values", profile)
	}
	if string(bundle.Unknown["rootUnknown"]) != `{"x":true}` || string(profile.Unknown["profileUnknown"]) != `[1,2]` {
		t.Fatalf("unknowns = %q / %q, want owned literal values", bundle.Unknown["rootUnknown"], profile.Unknown["profileUnknown"])
	}
	absent := bundle.Profiles[1]
	if absent.ID != nil || absent.Name != nil || absent.Version != nil || absent.Inputs == nil || len(absent.Inputs) != 0 {
		t.Fatalf("missing scalars/empty inputs = %#v, want nil scalars and non-nil empty inputs", absent)
	}
	input := profile.Inputs[0]
	wants := map[string]string{"macro": `{"steps":[1]}`, "number": "9223372036854775808", "flag": "true", "nothing": "null"}
	for key, want := range wants {
		if string(input[key]) != want {
			t.Fatalf("input[%q] = %q, want %q", key, input[key], want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		kind profiledecode.RootKind
		code ErrorCode
	}{
		{name: "malformed", raw: `{"profiles":`, kind: profiledecode.RootBundle, code: ERR_IMPORT_ROOT},
		{name: "nested duplicate", raw: `{"id":1,"inputs":[{"macro":{"a":1,"\u0061":2}}]}`, kind: profiledecode.RootSingle, code: ERR_IMPORT_ROOT},
		{name: "both roots", raw: `{"profiles":[],"id":1,"inputs":[]}`, kind: profiledecode.RootBundle, code: ERR_IMPORT_ROOT},
		{name: "neither root", raw: `{}`, kind: profiledecode.RootBundle, code: ERR_IMPORT_ROOT},
		{name: "unknown kind", raw: `{"profiles":[]}`, code: ERR_IMPORT_ROOT},
		{name: "kind mismatch", raw: `{"profiles":[]}`, kind: profiledecode.RootSingle, code: ERR_IMPORT_ROOT},
		{name: "root version object before profiles type", raw: `{"version":{},"profiles":null}`, kind: profiledecode.RootBundle, code: ERR_IMPORT_UNSUPPORTED_VERSION},
		{name: "root version array", raw: `{"version":[],"profiles":[]}`, kind: profiledecode.RootBundle, code: ERR_IMPORT_UNSUPPORTED_VERSION},
		{name: "profiles null", raw: `{"profiles":null}`, kind: profiledecode.RootBundle, code: ERR_IMPORT_ROOT},
		{name: "profile not object", raw: `{"profiles":[null]}`, kind: profiledecode.RootBundle, code: ERR_IMPORT_ROOT},
		{name: "profile missing inputs", raw: `{"profiles":[{}]}`, kind: profiledecode.RootBundle, code: ERR_IMPORT_ROOT},
		{name: "profile version before missing inputs", raw: `{"profiles":[{"version":{}}]}`, kind: profiledecode.RootBundle, code: ERR_IMPORT_UNSUPPORTED_VERSION},
		{name: "profile version before fields", raw: `{"profiles":[{"version":{},"id":[],"inputs":{}}]}`, kind: profiledecode.RootBundle, code: ERR_IMPORT_UNSUPPORTED_VERSION},
		{name: "container id", raw: `{"id":{},"inputs":[]}`, kind: profiledecode.RootSingle, code: ERR_IMPORT_ROOT},
		{name: "container name", raw: `{"name":[],"inputs":[]}`, kind: profiledecode.RootSingle, code: ERR_IMPORT_ROOT},
		{name: "inputs object", raw: `{"id":1,"inputs":{}}`, kind: profiledecode.RootSingle, code: ERR_IMPORT_ROOT},
		{name: "input not object", raw: `{"id":1,"inputs":[false]}`, kind: profiledecode.RootSingle, code: ERR_IMPORT_ROOT},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			export, err := Parse(profiledecode.Document{JSON: json.RawMessage(tt.raw), Kind: tt.kind})
			assertParseFailure(t, export, err, tt.code)
		})
	}
}

func assertParseFailure(t *testing.T, export RawExport, err error, want ErrorCode) {
	t.Helper()
	var parseError *ParseError
	if !errors.As(err, &parseError) || parseError.Code != want || err.Error() != string(want) {
		t.Fatalf("error = %T %v, want code-only %q", err, err, want)
	}
	if export != (RawExport{}) {
		t.Fatalf("RawExport = %#v, want zero value", export)
	}
	if bytes.Contains([]byte(err.Error()), []byte("macro")) {
		t.Fatalf("error %q leaks source data", err)
	}
}
