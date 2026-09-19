package control

import (
	"context"
	"errors"

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

func ResolveConfiguredProfile(ctx context.Context, source *profilesource.ImportedSource, settings config.Profile) (profilesource.Selection, profile.Profile, error) {
	if settings.Source == "local" {
		return profilesource.Selection{}, profile.Profile{}, NewError(ERR_PROFILE_SOURCE_UNAVAILABLE)
	}
	if settings.SelectedID != "" {
		selected, err := resolveProfile(ctx, source, settings.SelectedID, true)
		if err != nil {
			return profilesource.Selection{}, profile.Profile{}, err
		}
		return selected.selection, selected.profile, nil
	}
	selected, err := source.Selected(ctx)
	if err != nil {
		return profilesource.Selection{}, profile.Profile{}, err
	}
	bundle, err := source.Load(ctx, selected.Source)
	if err != nil {
		return profilesource.Selection{}, profile.Profile{}, err
	}
	return selected, bundle.Profiles[selected.ProfileIndex-1], nil
}

func resolveProfile(ctx context.Context, source *profilesource.ImportedSource, selector string, idOnly bool) (*activeSelection, error) {
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
	if errors.As(err, &failure) && failure.Code == profilesource.ERR_PROFILE_NOT_FOUND {
		return NewError(failure.Code)
	}
	return NewError(profilesource.ERR_PROFILE_STORAGE)
}
