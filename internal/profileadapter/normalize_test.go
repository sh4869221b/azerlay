package profileadapter_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
	"github.com/sh4869221b/azerlay/internal/profiledecode"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

func TestNormalizeAdmission(t *testing.T) {
	t.Parallel()

	admitted := profile.SourceMetadata{
		SoftwareRelease: "2.0.2",
		SourceScope:     "azeron-software-export",
	}

	t.Run("admitted empty bundle ignores raw version", func(t *testing.T) {
		t.Parallel()

		for _, sourceText := range []string{
			`{"profiles":[]}`,
			`{"version":null,"profiles":[]}`,
			`{"version":"2.0.3","profiles":[]}`,
			`{"version":999,"profiles":[]}`,
		} {
			raw := parseExport(t, sourceText)

			actual, err := profileadapter.Normalize(raw, admitted)

			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}
			want := profile.ProfileBundle{
				SchemaVersion: 1,
				Source:        admitted,
				RootKind:      profile.RootBundle,
				Profiles:      []profile.Profile{},
				Raw:           &profile.RawBundleReference{},
			}
			if !reflect.DeepEqual(actual, want) {
				t.Fatalf("Normalize() = %#v, want %#v", actual, want)
			}
		}
	})

	t.Run("admitted empty single has one profile", func(t *testing.T) {
		t.Parallel()

		raw := parseExport(t, `{"id":"synthetic","version":999,"inputs":[]}`)

		actual, err := profileadapter.Normalize(raw, admitted)

		if err != nil {
			t.Fatalf("Normalize() error = %v", err)
		}
		want := profile.ProfileBundle{
			SchemaVersion: 1,
			Source:        admitted,
			RootKind:      profile.RootSingle,
			Profiles: []profile.Profile{{
				Controls: []profile.ControlBinding{},
			}},
		}
		if !reflect.DeepEqual(actual, want) {
			t.Fatalf("Normalize() = %#v, want %#v", actual, want)
		}
	})

	raw := parseExport(t, `{"version":"2.0.2","profiles":[]}`)
	for _, test := range []struct {
		name   string
		source profile.SourceMetadata
	}{
		{name: "missing release", source: profile.SourceMetadata{SourceScope: admitted.SourceScope}},
		{name: "release family", source: profile.SourceMetadata{SoftwareRelease: "2.x", SourceScope: admitted.SourceScope}},
		{name: "different patch", source: profile.SourceMetadata{SoftwareRelease: "2.0.3", SourceScope: admitted.SourceScope}},
		{name: "release whitespace", source: profile.SourceMetadata{SoftwareRelease: " 2.0.2", SourceScope: admitted.SourceScope}},
		{name: "wrong scope", source: profile.SourceMetadata{SoftwareRelease: admitted.SoftwareRelease, SourceScope: "other"}},
		{name: "missing scope", source: profile.SourceMetadata{SoftwareRelease: admitted.SoftwareRelease}},
	} {
		t.Run("unadmitted source "+test.name, func(t *testing.T) {
			t.Parallel()

			actual, err := profileadapter.Normalize(raw, test.source)

			if !reflect.DeepEqual(actual, profile.ProfileBundle{}) {
				t.Fatalf("Normalize() result = %#v, want zero bundle", actual)
			}
			var normalizeError *profileadapter.NormalizeError
			if !errors.As(err, &normalizeError) {
				t.Fatalf("Normalize() error = %T %v, want *NormalizeError", err, err)
			}
			if normalizeError.Code != profileadapter.ERR_IMPORT_UNSUPPORTED_VERSION {
				t.Fatalf("NormalizeError.Code = %q, want %q", normalizeError.Code, profileadapter.ERR_IMPORT_UNSUPPORTED_VERSION)
			}
			if err.Error() != string(profileadapter.ERR_IMPORT_UNSUPPORTED_VERSION) {
				t.Fatalf("error text = %q, want code only", err.Error())
			}
		})
	}
}

func parseExport(t *testing.T, source string) profileraw.RawExport {
	t.Helper()

	document, err := profiledecode.DecodeText(source)
	if err != nil {
		t.Fatalf("DecodeText() error = %v", err)
	}
	export, err := profileraw.Parse(document)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	return export
}
