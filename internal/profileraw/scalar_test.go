package profileraw

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestRawScalar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    RawScalar
		wantErr bool
		direct  bool
	}{
		{name: "number_exponent", raw: "1e+03", want: RawScalar{Kind: ScalarNumber, Raw: json.RawMessage("1e+03"), Number: json.Number("1e+03")}},
		{name: "number_fraction", raw: "1.25", want: RawScalar{Kind: ScalarNumber, Raw: json.RawMessage("1.25"), Number: json.Number("1.25")}},
		{name: "number_overflow", raw: "9223372036854775808", want: RawScalar{Kind: ScalarNumber, Raw: json.RawMessage("9223372036854775808"), Number: json.Number("9223372036854775808")}},
		{name: "number_negative_zero", raw: "-0", want: RawScalar{Kind: ScalarNumber, Raw: json.RawMessage("-0"), Number: json.Number("-0")}},
		{name: "string_numeric", raw: `"001"`, want: RawScalar{Kind: ScalarString, Raw: json.RawMessage(`"001"`), String: "001"}},
		{name: "string_escape", raw: `"0\u00301"`, want: RawScalar{Kind: ScalarString, Raw: json.RawMessage(`"0\u00301"`), String: "001"}},
		{name: "boolean", raw: "true", want: RawScalar{Kind: ScalarBool, Raw: json.RawMessage("true"), Bool: true}},
		{name: "null", raw: "null", want: RawScalar{Kind: ScalarNull, Raw: json.RawMessage("null")}},
		{name: "reject_object", raw: `{}`, wantErr: true},
		{name: "reject_array", raw: `[]`, wantErr: true},
		{name: "reject_malformed", raw: `tru`, wantErr: true},
		{name: "reject_trailing", raw: `true false`, wantErr: true},
		{name: "reject_vertical_tab_whitespace", raw: "\v1", wantErr: true, direct: true},
		{name: "reject_nbsp_whitespace", raw: "\u00a01\u00a0", wantErr: true, direct: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Given
			source := []byte(" \t" + tt.raw + "\n")
			if tt.direct {
				source = []byte(tt.raw)
			}
			scalar := RawScalar{Kind: ScalarString, Raw: json.RawMessage(`"prior"`), String: "prior"}

			// When
			err := scalar.UnmarshalJSON(source)

			// Then
			if tt.wantErr {
				var parseError *ParseError
				if !errors.As(err, &parseError) {
					t.Fatalf("error = %T %v, want *ParseError", err, err)
				}
				if parseError.Code != ERR_IMPORT_ROOT || err.Error() != string(ERR_IMPORT_ROOT) {
					t.Fatalf("error = %#v (%q), want code-only %q", parseError, err, ERR_IMPORT_ROOT)
				}
				if scalar.Kind != ScalarNull || scalar.Raw != nil || scalar.Bool || scalar.String != "" || scalar.Number != "" {
					t.Fatalf("RawScalar = %#v, want zero value", scalar)
				}
				return
			}
			if err != nil {
				t.Fatalf("UnmarshalJSON() error = %v", err)
			}
			for index := range source {
				source[index] = 'x'
			}
			if scalar.Kind != tt.want.Kind || scalar.Bool != tt.want.Bool || scalar.String != tt.want.String || scalar.Number != tt.want.Number || string(scalar.Raw) != string(tt.want.Raw) {
				t.Fatalf("RawScalar = %#v, want %#v", scalar, tt.want)
			}
		})
	}
}
