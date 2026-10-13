package profileraw

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/sh4869221b/azerlay/internal/profiledecode"
)

func TestParseBundleOpaqueInputsIndependentOfHint(t *testing.T) {
	t.Parallel()
	inputs := `[` + repeatedMembers(`{}`, maxInputs+1) + `]`
	for _, root := range []string{
		`{"profiles":[],"inputs":` + inputs + `}`,
		`{"inputs":` + inputs + `,"profiles":[]}`,
		`{"\u0069nputs":` + inputs + `,"pro\u0066iles":[]}`,
	} {
		for _, hint := range []profiledecode.RootKind{profiledecode.RootBundle, profiledecode.RootSingle, "", "unknown"} {
			t.Run(fmt.Sprintf("%s/%s", root[:20], hint), func(t *testing.T) {
				t.Parallel()
				export, err := Parse(profiledecode.Document{JSON: json.RawMessage(root), Kind: hint})
				if hint != profiledecode.RootBundle {
					assertParseFailure(t, export, err, ERR_IMPORT_ROOT)
					return
				}
				if err != nil || export.Bundle == nil || !bytes.Equal(export.Bundle.Unknown["inputs"], []byte(inputs)) {
					t.Fatalf("bundle opaque inputs weren't preserved: error %v", err)
				}
			})
		}
	}
}

func TestParseModeledArrayLimitsIndependentOfHint(t *testing.T) {
	t.Parallel()
	inputs := `[` + repeatedMembers(`{}`, maxInputs+1) + `]`
	profiles := `[` + repeatedMembers(`{"inputs":[]}`, maxProfiles+1) + `]`
	for _, root := range []string{
		`{"id":null,"inputs":` + inputs + `}`,
		`{"inputs":` + inputs + `,"name":null}`,
		`{"\u0069nputs":` + inputs + `,"\u0069d":null}`,
		`{"profiles":` + profiles + `}`,
	} {
		for _, hint := range []profiledecode.RootKind{profiledecode.RootBundle, profiledecode.RootSingle, "", "unknown"} {
			t.Run(fmt.Sprintf("%s/%s", root[:20], hint), func(t *testing.T) {
				t.Parallel()
				export, err := Parse(profiledecode.Document{JSON: json.RawMessage(root), Kind: hint})
				assertParseFailure(t, export, err, ERR_IMPORT_LIMIT_EXCEEDED)
			})
		}
	}
}

func TestParseInvalidRootDoesNotModelArrays(t *testing.T) {
	t.Parallel()
	inputs := `[` + repeatedMembers(`{}`, maxInputs+1) + `]`
	for _, root := range []string{
		`{"inputs":` + inputs + `}`,
		`{"profiles":[],"id":null,"inputs":` + inputs + `}`,
	} {
		for _, hint := range []profiledecode.RootKind{profiledecode.RootBundle, profiledecode.RootSingle, "", "unknown"} {
			t.Run(fmt.Sprintf("%s/%s", root[:20], hint), func(t *testing.T) {
				t.Parallel()
				export, err := Parse(profiledecode.Document{JSON: json.RawMessage(root), Kind: hint})
				assertParseFailure(t, export, err, ERR_IMPORT_ROOT)
			})
		}
	}
}

func TestParseRootScopeLeavesPreflightErrorsInOrder(t *testing.T) {
	t.Parallel()
	inputs := repeatedMembers(`{}`, maxInputs)
	profiles := repeatedMembers(`{"inputs":[]}`, maxProfiles)
	for _, tc := range []struct {
		name, root string
		code       ErrorCode
	}{
		{"malformed input tail", `{"id":null,"inputs":[` + inputs + `,{"private":"`, ERR_IMPORT_LIMIT_EXCEEDED},
		{"malformed profile tail", `{"profiles":[` + profiles + `,{"private":"`, ERR_IMPORT_LIMIT_EXCEEDED},
		{"incomplete bundle context", `{"profiles":[],"inputs":[` + inputs + `,{"private":"`, ERR_IMPORT_LIMIT_EXCEEDED},
		{"invalid UTF8 tail", `{"id":null,"inputs":[` + inputs + `,{"private":"` + string([]byte{0xff}), ERR_IMPORT_ROOT},
		{"duplicate before invalid root", `{"inputs":[{"private":1,"private":2}]}`, ERR_IMPORT_ROOT},
		{"string limit before invalid root", `{"inputs":["` + string(bytes.Repeat([]byte("x"), maxJSONStringBytes+1)) + `"]}`, ERR_IMPORT_LIMIT_EXCEEDED},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			export, err := Parse(profiledecode.Document{JSON: json.RawMessage(tc.root), Kind: profiledecode.RootBundle})
			assertParseFailure(t, export, err, tc.code)
		})
	}
}

func TestParseRootScopeIgnoresNestedAndQuotedKeys(t *testing.T) {
	t.Parallel()
	inputs := `[` + repeatedMembers(`{}`, maxInputs+1) + `]`
	root := json.RawMessage(`{"other":{"profiles":[],"inputs":` + inputs + `},"text":"\"profiles\":[],{}[]\\","id":null,"inputs":[]}`)
	export, err := Parse(profiledecode.Document{JSON: root, Kind: profiledecode.RootSingle})
	if err != nil || export.Single == nil || len(export.Single.Inputs) != 0 {
		t.Fatalf("nested/quoted root keys affected scope: error %v", err)
	}
}

func FuzzRootContext(f *testing.F) {
	for _, seed := range []string{
		`{"profiles":[],"inputs":[]}`,
		`{"inputs":[],"name":null}`,
		`{"profiles":[],"inputs":[],"id":null}`,
		`{"other":{"profiles":[]},"id":null,"inputs":[]}`,
		`{"\u0070\u0072\u006f\u0066\u0069\u006c\u0065\u0073":[],"\u0069nputs":[]}`,
		`{"text":"\"profiles\":[],{}[]\\","inputs":[],"n\u0061me":null}`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, source []byte) {
		// Bound the independent stdlib oracle, which materializes values; the
		// production key inspection uses raw spans without those allocations.
		if len(source) > 64*1024 {
			return
		}
		got := preflightRootContext(source)
		if !utf8.Valid(source) || !json.Valid(source) {
			return
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(source, &fields) != nil || fields == nil {
			return
		}
		_, profiles := fields["profiles"]
		_, inputs := fields["inputs"]
		_, id := fields["id"]
		_, name := fields["name"]
		single := inputs && (id || name)
		want := opaqueJSON
		if profiles && !single {
			want = bundleJSON
		} else if single && !profiles {
			want = profileJSON
		}
		if got != want {
			t.Fatalf("root context = %d, want %d from stdlib key presence", got, want)
		}
	})
}
