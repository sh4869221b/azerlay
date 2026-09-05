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
		profiles := make([]profile.Profile, len(raw.Bundle.Profiles))
		for index, rawProfile := range raw.Bundle.Profiles {
			profiles[index] = normalizeProfile(rawProfile, profile.RootBundle, index)
		}
		return profile.ProfileBundle{
			SchemaVersion: 1,
			Source:        source,
			RootKind:      profile.RootBundle,
			Profiles:      profiles,
			Raw: &profile.RawBundleReference{
				Version: cloneScalarRaw(raw.Bundle.Version),
				Unknown: cloneRawMap(raw.Bundle.Unknown),
			},
		}, nil
	}

	return profile.ProfileBundle{
		SchemaVersion: 1,
		Source:        source,
		RootKind:      profile.RootSingle,
		Profiles: []profile.Profile{
			normalizeProfile(*raw.Single, profile.RootSingle, 0),
		},
	}, nil
}

func normalizeProfile(raw profileraw.RawProfile, rootKind profile.RootKind, profileIndex int) profile.Profile {
	normalized := preserveProfile(raw, rootKind, profileIndex)
	for inputIndex, input := range raw.Inputs {
		normalized.Controls[inputIndex].Bindings = normalizeKeyboard(input, normalized.Controls[inputIndex].Bindings)
	}
	return normalized
}
