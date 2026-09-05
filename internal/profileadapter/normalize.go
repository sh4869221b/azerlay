package profileadapter

import (
	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

// Normalize converts an admitted parsed export into the version-independent model.
func Normalize(raw profileraw.RawExport, source profile.SourceMetadata) (profile.ProfileBundle, error) {
	if source.SoftwareRelease != "2.0.2" || source.SourceScope != "azeron-software-export" {
		return profile.ProfileBundle{}, &NormalizeError{Code: ERR_IMPORT_UNSUPPORTED_VERSION}
	}

	if raw.Bundle != nil {
		return profile.ProfileBundle{
			SchemaVersion: 1,
			Source:        source,
			RootKind:      profile.RootBundle,
			Profiles:      []profile.Profile{},
			Raw:           &profile.RawBundleReference{},
		}, nil
	}

	return profile.ProfileBundle{
		SchemaVersion: 1,
		Source:        source,
		RootKind:      profile.RootSingle,
		Profiles: []profile.Profile{{
			Controls: []profile.ControlBinding{},
		}},
	}, nil
}
