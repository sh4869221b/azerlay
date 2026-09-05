package profileadapter_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
	"github.com/sh4869221b/azerlay/internal/profiledecode"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

func FuzzNormalize(f *testing.F) {
	f.Add([]byte(`{"profiles":[]}`), true)
	f.Add([]byte(`{"id":"seed","inputs":[{"types":null,"opaque":-0}]}`), true)
	f.Add([]byte(`{"id":"seed","inputs":[]}`), false)
	f.Add([]byte{0xff}, true)

	f.Fuzz(func(t *testing.T, sourceBytes []byte, admitted bool) {
		document, err := profiledecode.DecodeText(string(sourceBytes))
		if err != nil {
			var decodeError *profiledecode.DecodeError
			if !errors.As(err, &decodeError) || !reflect.DeepEqual(document, profiledecode.Document{}) {
				t.Fatalf("DecodeText() = %#v, %T %v; want typed error and zero document", document, err, err)
			}
			return
		}

		raw, err := profileraw.Parse(document)
		if err != nil {
			var parseError *profileraw.ParseError
			if !errors.As(err, &parseError) || raw != (profileraw.RawExport{}) {
				t.Fatalf("Parse() = %#v, %T %v; want typed error and zero export", raw, err, err)
			}
			return
		}

		source := admittedSource
		if !admitted {
			source = profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "unadmitted"}
		}
		bundle, err := profileadapter.Normalize(raw, source)
		if err != nil {
			var normalizeError *profileadapter.NormalizeError
			if !errors.As(err, &normalizeError) || !reflect.DeepEqual(bundle, profile.ProfileBundle{}) || err.Error() != string(normalizeError.Code) {
				t.Fatalf("Normalize() = %#v, %T %v; want typed code-only error and zero bundle", bundle, err, err)
			}
			if !admitted && normalizeError.Code != profileadapter.ERR_IMPORT_UNSUPPORTED_VERSION {
				t.Fatalf("unadmitted error code = %q", normalizeError.Code)
			}
			if admitted && normalizeError.Code != profileadapter.ERR_IMPORT_LIMIT_EXCEEDED {
				t.Fatalf("admitted error code = %q", normalizeError.Code)
			}
			return
		}
		if !admitted {
			t.Fatal("unadmitted source normalized successfully")
		}
		assertFuzzRetention(t, raw, bundle, source)
	})
}

func assertFuzzRetention(t *testing.T, raw profileraw.RawExport, bundle profile.ProfileBundle, source profile.SourceMetadata) {
	t.Helper()
	if bundle.SchemaVersion != 1 || bundle.Source != source {
		t.Fatalf("normalized metadata = schema %d source %#v", bundle.SchemaVersion, bundle.Source)
	}

	var rawProfiles []profileraw.RawProfile
	switch {
	case raw.Bundle != nil:
		if bundle.RootKind != profile.RootBundle || bundle.Raw == nil || !bytes.Equal(bundle.Raw.Version, scalarToken(raw.Bundle.Version)) || !sameRawFields(bundle.Raw.Unknown, raw.Bundle.Unknown) {
			t.Fatalf("bundle root/raw retention = %#v", bundle)
		}
		rawProfiles = raw.Bundle.Profiles
	case raw.Single != nil:
		if bundle.RootKind != profile.RootSingle || bundle.Raw != nil {
			t.Fatalf("single root retention = %#v", bundle)
		}
		rawProfiles = []profileraw.RawProfile{*raw.Single}
	default:
		t.Fatal("successful parse had no root")
	}
	if len(bundle.Profiles) != len(rawProfiles) {
		t.Fatalf("profile count = %d, want %d", len(bundle.Profiles), len(rawProfiles))
	}
	for profileIndex, rawProfile := range rawProfiles {
		normalizedProfile := bundle.Profiles[profileIndex]
		if !bytes.Equal(normalizedProfile.Raw.ID, scalarToken(rawProfile.ID)) || !bytes.Equal(normalizedProfile.Raw.Name, scalarToken(rawProfile.Name)) || !bytes.Equal(normalizedProfile.Raw.Version, scalarToken(rawProfile.Version)) || !sameRawFields(normalizedProfile.Raw.Unknown, rawProfile.Unknown) {
			t.Fatalf("profile %d raw retention = %#v", profileIndex, normalizedProfile.Raw)
		}
		if len(normalizedProfile.Controls) != len(rawProfile.Inputs) {
			t.Fatalf("profile %d control count = %d, want %d", profileIndex, len(normalizedProfile.Controls), len(rawProfile.Inputs))
		}
		for inputIndex, rawInput := range rawProfile.Inputs {
			control := normalizedProfile.Controls[inputIndex]
			if control.Raw.RootKind != bundle.RootKind || control.Raw.ProfileIndex != profileIndex || control.Raw.InputIndex != inputIndex || !sameRawFields(control.Raw.Fields, map[string]json.RawMessage(rawInput)) {
				t.Fatalf("control %d/%d provenance/raw retention = %#v", profileIndex, inputIndex, control.Raw)
			}
		}
	}
}

func sameRawFields(actual, expected map[string]json.RawMessage) bool {
	if len(actual) != len(expected) {
		return false
	}
	for key, expectedValue := range expected {
		actualValue, ok := actual[key]
		if !ok || !bytes.Equal(actualValue, expectedValue) {
			return false
		}
	}
	return true
}

func scalarToken(scalar *profileraw.RawScalar) []byte {
	if scalar == nil {
		return nil
	}
	return scalar.Raw
}
