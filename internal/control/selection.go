package control

import (
	"context"
	"errors"
	"path"
	"sort"

	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profilesource"
)

type activeSelection struct {
	selection profilesource.Selection
	profile   profile.Profile
}

func (a *activeSelection) status() ProfileStatus {
	result := ProfileStatus{Source: "imported", SourceRef: a.selection.Source.Hash, ProfileIndex: a.selection.ProfileIndex}
	if ref := a.selection.Source.Local; ref != (profilesource.LocalRef{}) {
		result.Source = "local"
		result.SourceRef = path.Join("Storage", "DevicesStorage", ref.Device, "ProfileStorage", ref.File)
	}
	if a.profile.Name != nil {
		name := *a.profile.Name
		result.Name = &name
	}
	return result
}

func (c *Controller) resolve(ctx context.Context, selector string, idOnly bool) (*activeSelection, *Error) {
	selected, err := resolveProfile(ctx, c.source, selector, idOnly)
	if err != nil {
		return nil, selectionError(err)
	}
	return selected, nil
}

// ResolveConfiguredProfile does not watch or persist. A valid retained local
// profile or imported fallback may accompany a local error for doctor to report
// availability and degradation together. Other failures return no selection.
func ResolveConfiguredProfile(ctx context.Context, source *profilesource.ImportedSource, settings config.Profile) (profilesource.Selection, profile.Profile, error) {
	if settings.SelectedID != "" {
		selected, err := resolveProfile(ctx, source, settings.SelectedID, true)
		if err != nil {
			return profilesource.Selection{}, profile.Profile{}, err
		}
		return selected.selection, selected.profile, nil
	}
	var localFailure error
	if settings.Source != "imported" && settings.LocalDevice != "" {
		ref, err := profilesource.ResolveLocalRef(settings.LocalStorePath, settings.LocalDevice, settings.LocalProfileFile)
		if err == nil {
			candidate, loadErr := profilesource.NewAzeronLocalSource(ref.Root).LoadCandidate(ctx, profilesource.SourceRef{Local: ref})
			if loadErr == nil {
				selected := localActive(ref, candidate)
				return selected.selection, selected.profile, nil
			}
			err = loadErr
		}
		if ctx.Err() != nil {
			return profilesource.Selection{}, profile.Profile{}, ctx.Err()
		}
		localFailure = localReadError(err)
		if ref, candidate, restoreErr := profilesource.RestoreLocalState(localStateSelection(settings)); restoreErr == nil {
			selected := localActive(ref, candidate)
			return selected.selection, selected.profile, localFailure
		}
		if settings.Source == "local" {
			return profilesource.Selection{}, profile.Profile{}, localReadError(err)
		}
	}
	if settings.Source == "local" {
		return profilesource.Selection{}, profile.Profile{}, NewError(ERR_PROFILE_SOURCE_UNAVAILABLE)
	}
	selected, err := resolveSavedImported(ctx, source, settings.Source == "auto")
	if err != nil {
		return profilesource.Selection{}, profile.Profile{}, err
	}
	return selected.selection, selected.profile, localFailure
}

func resolveSavedImported(ctx context.Context, source *profilesource.ImportedSource, fallback bool) (*activeSelection, error) {
	if source == nil {
		return nil, NewError(ERR_PROFILE_SOURCE_UNAVAILABLE)
	}
	selected, err := source.Selected(ctx)
	if err == nil {
		bundle, loadErr := source.Load(ctx, selected.Source)
		if loadErr == nil {
			return &activeSelection{selection: selected, profile: bundle.Profiles[selected.ProfileIndex-1]}, nil
		}
		err = loadErr
	}
	if !fallback || ctx.Err() != nil {
		return nil, err
	}
	sources, discoverErr := source.Discover(ctx)
	if discoverErr != nil {
		return nil, discoverErr
	}
	// Preserve catalog order for equal timestamps, preferring the later entry.
	for left, right := 0, len(sources)-1; left < right; left, right = left+1, right-1 {
		sources[left], sources[right] = sources[right], sources[left]
	}
	sort.SliceStable(sources, func(i, j int) bool { return sources[i].ImportedAt.After(sources[j].ImportedAt) })
	for _, descriptor := range sources {
		if descriptor.ProfileCount != 1 {
			continue
		}
		bundle, loadErr := source.Load(ctx, descriptor.Ref)
		if loadErr == nil {
			return &activeSelection{selection: profilesource.Selection{Source: descriptor.Ref, ProfileIndex: 1}, profile: bundle.Profiles[0]}, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, err
}

func resolveProfile(ctx context.Context, source *profilesource.ImportedSource, selector string, idOnly bool) (*activeSelection, error) {
	if source == nil {
		return nil, NewError(ERR_PROFILE_SOURCE_UNAVAILABLE)
	}
	sources, err := source.Discover(ctx)
	if err != nil {
		return nil, err
	}
	var candidate *activeSelection
	matches := 0
	for _, descriptor := range sources {
		bundle, err := source.Load(ctx, descriptor.Ref)
		if err != nil {
			return nil, err
		}
		for i, p := range bundle.Profiles {
			if p.ID != nil && *p.ID == selector || !idOnly && p.Name != nil && *p.Name == selector {
				matches++
				candidate = &activeSelection{selection: profilesource.Selection{Source: descriptor.Ref, ProfileIndex: i + 1}, profile: p}
			}
		}
	}
	if ctx.Err() != nil {
		return nil, NewError(ERR_CONTROL_UNAVAILABLE)
	}
	switch matches {
	case 0:
		return nil, NewError(profilesource.ERR_PROFILE_NOT_FOUND)
	case 1:
		return candidate, nil
	default:
		return nil, NewError(ERR_PROFILE_AMBIGUOUS)
	}
}

func selectionError(err error) *Error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return NewError(ERR_CONTROL_UNAVAILABLE)
	}
	var controlFailure *Error
	if errors.As(err, &controlFailure) {
		return controlFailure
	}
	var failure *profilesource.Error
	if errors.As(err, &failure) {
		switch failure.Code {
		case profilesource.ERR_PROFILE_NOT_FOUND, profilesource.ERR_PROFILE_LOCAL_READ, profilesource.ERR_PROFILE_LOCAL_UNSUPPORTED,
			profilesource.ERR_PROFILE_LOCAL_STATE, profilesource.ERR_PROFILE_LOCAL_WATCH:
			return NewError(failure.Code)
		}
	}
	return NewError(profilesource.ERR_PROFILE_STORAGE)
}
