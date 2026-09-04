package profileraw_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profiledecode"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

func FuzzParse(f *testing.F) {
	f.Add([]byte(`{"profiles":[]}`), string(profiledecode.RootBundle))
	f.Add([]byte(`{"id":null,"inputs":[]}`), string(profiledecode.RootSingle))
	f.Add([]byte(`{"version":{},"profiles":[]}`), string(profiledecode.RootBundle))
	f.Add([]byte(`{"profiles":[]}`), "provisional-unknown")
	f.Add([]byte{0xff}, "")

	f.Fuzz(func(t *testing.T, rawJSON []byte, provisionalKind string) {
		// Given
		document := profiledecode.Document{
			JSON: json.RawMessage(rawJSON),
			Kind: profiledecode.RootKind(provisionalKind),
		}

		// When
		export, err := profileraw.Parse(document)

		// Then
		if err == nil {
			if (export.Bundle == nil) == (export.Single == nil) {
				t.Fatalf("successful RawExport = %#v, want exactly one root", export)
			}
			return
		}
		var parseError *profileraw.ParseError
		if !errors.As(err, &parseError) {
			t.Fatalf("error type = %T, want *ParseError", err)
		}
		switch parseError.Code {
		case profileraw.ERR_IMPORT_ROOT,
			profileraw.ERR_IMPORT_LIMIT_EXCEEDED,
			profileraw.ERR_IMPORT_UNSUPPORTED_VERSION:
		default:
			t.Fatalf("error code = %q, want stable ParseError code", parseError.Code)
		}
		if err.Error() != string(parseError.Code) {
			t.Fatalf("error text = %q, want %q", err, parseError.Code)
		}
		if export != (profileraw.RawExport{}) {
			t.Fatalf("failed RawExport = %#v, want zero value", export)
		}
	})
}
