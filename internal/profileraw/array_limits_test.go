package profileraw

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profiledecode"
)

func TestValidateJSONStopsAtModeledArrayLimits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prefix string
	}{
		{
			name:   "profiles",
			prefix: `{"profiles":[` + repeatedMembers(`{"inputs":[]}`, maxProfiles),
		},
		{
			name:   "single inputs",
			prefix: `{"id":null,"inputs":[` + repeatedMembers(`{}`, maxInputs),
		},
		{
			name:   "bundle member inputs",
			prefix: `{"profiles":[{"inputs":[` + repeatedMembers(`{}`, maxInputs),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// An unread, malformed tail makes consuming the oversized member
			// observable without relying on elapsed time or process memory.
			raw := tt.prefix + `,{"private":"` + strings.Repeat("x", 128*1024)
			decoder := json.NewDecoder(strings.NewReader(raw))
			decoder.UseNumber()
			opening, err := decoder.Token()
			if err != nil {
				t.Fatalf("decoder.Token() error = %v", err)
			}
			err = validateJSONToken(decoder, opening, 0, rootJSON)
			assertParseFailure(t, RawExport{}, err, ERR_IMPORT_LIMIT_EXCEEDED)
			if decoder.InputOffset() != int64(len(tt.prefix)) {
				t.Fatalf("consumed %d bytes, want stop after %d-byte allowed prefix", decoder.InputOffset(), len(tt.prefix))
			}
		})
	}
}

func TestParseArrayRejectsInvalidAndTrailingValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		code ErrorCode
	}{
		{name: "absent", code: ERR_IMPORT_ROOT},
		{name: "null", raw: `null`, code: ERR_IMPORT_ROOT},
		{name: "object", raw: `{}`, code: ERR_IMPORT_ROOT},
		{name: "string", raw: `"[]"`, code: ERR_IMPORT_ROOT},
		{name: "unterminated", raw: `[{} `, code: ERR_IMPORT_ROOT},
		{name: "trailing comma", raw: `[{},]`, code: ERR_IMPORT_ROOT},
		{name: "second value", raw: `[] {}`, code: ERR_IMPORT_ROOT},
		{name: "trailing garbage", raw: `[] private`, code: ERR_IMPORT_ROOT},
		{name: "empty", raw: `[]`},
		{name: "exact limit", raw: `[{},{}]`},
		{name: "overflow", raw: `[{}, {}, {}]`, code: ERR_IMPORT_LIMIT_EXCEEDED},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			values, err := parseArray(json.RawMessage(tt.raw), 2)
			if tt.code == "" {
				if err != nil || values == nil {
					t.Fatalf("parseArray() = %v, %v, want non-nil accepted values", values, err)
				}
				return
			}
			var parseError *ParseError
			if !errors.As(err, &parseError) || parseError.Code != tt.code || err.Error() != string(tt.code) || values != nil {
				t.Fatalf("parseArray() = %v, %v, want nil values and code-only %s", values, err, tt.code)
			}
		})
	}
}

func TestParseOversizedArraysHaveBoundedAllocations(t *testing.T) {
	// AllocsPerRun owns this test's allocation measurement; don't run it in
	// parallel with the subtests below or use process-wide memory readings.
	tests := []struct {
		name   string
		prefix string
		member string
		suffix string
		limit  int
		kind   profiledecode.RootKind
	}{
		{name: "profiles", prefix: `{"profiles":[`, member: `{"inputs":[]}`, suffix: `]}`, limit: maxProfiles, kind: profiledecode.RootBundle},
		{name: "single inputs", prefix: `{"id":null,"inputs":[`, member: `{}`, suffix: `]}`, limit: maxInputs, kind: profiledecode.RootSingle},
		{name: "bundle member inputs", prefix: `{"profiles":[{"inputs":[`, member: `{}`, suffix: `]}]}`, limit: maxInputs, kind: profiledecode.RootBundle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			small := profiledecode.Document{JSON: json.RawMessage(tt.prefix + repeatedMembers(tt.member, tt.limit+1) + tt.suffix), Kind: tt.kind}
			large := profiledecode.Document{JSON: json.RawMessage(tt.prefix + repeatedMembers(tt.member, 32*tt.limit) + tt.suffix), Kind: tt.kind}
			for _, document := range []profiledecode.Document{small, large} {
				export, err := Parse(document)
				assertParseFailure(t, export, err, ERR_IMPORT_LIMIT_EXCEEDED)
			}
			measure := func(document profiledecode.Document) float64 {
				return testing.AllocsPerRun(3, func() {
					_, _ = Parse(document)
				})
			}
			smallAllocs, largeAllocs := measure(small), measure(large)
			if largeAllocs > smallAllocs+16 {
				t.Fatalf("allocations grew from %.0f to %.0f for a larger rejected tail", smallAllocs, largeAllocs)
			}
		})
	}
}

func TestParseBundleInputLimits(t *testing.T) {
	t.Parallel()
	for _, count := range []int{maxInputs, maxInputs + 1} {
		inputs := `[` + repeatedMembers(`{}`, count) + `]`
		profile := `{"inputs":` + inputs + `}`
		raw := json.RawMessage(`{"profiles":[` + profile + `,` + profile + `]}`)
		export, err := Parse(profiledecode.Document{JSON: raw, Kind: profiledecode.RootBundle})
		if count > maxInputs {
			assertParseFailure(t, export, err, ERR_IMPORT_LIMIT_EXCEEDED)
			continue
		}
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if len(export.Bundle.Profiles) != 2 || len(export.Bundle.Profiles[0].Inputs) != count || len(export.Bundle.Profiles[1].Inputs) != count {
			t.Fatal("input count wasn't enforced separately for each profile")
		}
	}
}

func TestParseCountLimitIsAPreflightFailure(t *testing.T) {
	t.Parallel()
	inputs := repeatedMembers(`{}`, maxInputs)
	tests := []struct {
		name string
		raw  string
		code ErrorCode
	}{
		{
			name: "duplicate before overflow",
			raw:  `{"id":null,"inputs":[{"private":1,"private":2},` + inputs + `]}`,
			code: ERR_IMPORT_ROOT,
		},
		{
			name: "duplicate after overflow",
			raw:  `{"id":null,"inputs":[` + inputs + `,{"private":1,"private":2}]}`,
			code: ERR_IMPORT_LIMIT_EXCEEDED,
		},
		{
			name: "unsupported version before overflow",
			raw:  `{"id":null,"version":{},"inputs":[` + inputs + `,{}]}`,
			code: ERR_IMPORT_LIMIT_EXCEEDED,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			export, err := Parse(profiledecode.Document{JSON: json.RawMessage(tt.raw), Kind: profiledecode.RootSingle})
			assertParseFailure(t, export, err, tt.code)
		})
	}
}

func TestParseOpaqueArrayNamesRemainUnboundedByModelCounts(t *testing.T) {
	t.Parallel()
	profiles := `[` + repeatedMembers(`{}`, maxProfiles+1) + `]`
	inputs := `[` + repeatedMembers(`{}`, maxInputs+1) + `]`
	opaque := `{"profiles":` + profiles + `,"inputs":` + inputs + `}`
	raw := json.RawMessage(`{"profiles":[{"inputs":[` + opaque + `],"profiles":` + profiles + `,"other":` + opaque + `}],"inputs":` + inputs + `,"other":` + opaque + `}`)
	export, err := Parse(profiledecode.Document{JSON: raw, Kind: profiledecode.RootBundle})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	input := export.Bundle.Profiles[0].Inputs[0]
	if !bytes.Equal(input["profiles"], []byte(profiles)) || !bytes.Equal(input["inputs"], []byte(inputs)) || !bytes.Equal(export.Bundle.Unknown["inputs"], []byte(inputs)) || !bytes.Equal(export.Bundle.Unknown["other"], []byte(opaque)) || !bytes.Equal(export.Bundle.Profiles[0].Unknown["profiles"], []byte(profiles)) || !bytes.Equal(export.Bundle.Profiles[0].Unknown["other"], []byte(opaque)) {
		t.Fatal("opaque arrays weren't preserved")
	}
}

func repeatedMembers(member string, count int) string {
	return strings.TrimSuffix(strings.Repeat(member+",", count), ",")
}
