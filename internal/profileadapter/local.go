package profileadapter

import (
	"bytes"
	"encoding/json"
	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

// NormalizeLocal admits the observed saved Software JSON subset. The raw value
// must come from profileraw.Parse, which enforces complete JSON structural limits.
func NormalizeLocal(raw profileraw.RawExport, basename string) (profile.ProfileBundle, error) {
	unsupported := func() (profile.ProfileBundle, error) {
		return profile.ProfileBundle{}, &NormalizeError{Code: ERR_IMPORT_UNSUPPORTED_VERSION}
	}
	p := raw.Single
	if raw.Bundle != nil || p == nil || p.ID == nil || p.ID.Kind != profileraw.ScalarString || p.ID.String == "" || basename != "profile_"+p.ID.String+".json" || p.Name == nil || p.Name.Kind != profileraw.ScalarString || p.Version == nil || p.Version.Kind != profileraw.ScalarNumber || p.Version.Number.String() != "1" {
		return unsupported()
	}
	if !bytes.Equal(bytes.TrimSpace(p.Unknown["isSoftware"]), []byte("true")) {
		return unsupported()
	}
	var metadata map[string]json.RawMessage
	if json.Unmarshal(p.Unknown["metaData"], &metadata) != nil || metadata == nil {
		return unsupported()
	}
	var logs []map[string]json.RawMessage
	if json.Unmarshal(metadata["changedLogs"], &logs) != nil || len(logs) == 0 {
		return unsupported()
	}
	for _, log := range logs {
		var version string
		if json.Unmarshal(log["softwareVersion"], &version) != nil || version != "2.0.2" {
			return unsupported()
		}
	}
	normalized, err := normalizeProfile(*p, profile.RootSingle, 0)
	if err != nil {
		return profile.ProfileBundle{}, err
	}
	return profile.ProfileBundle{SchemaVersion: 1, Source: profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-local-json"}, RootKind: profile.RootSingle, Profiles: []profile.Profile{normalized}}, nil
}
