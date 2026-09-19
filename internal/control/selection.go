package control

import (
	"context"
	"errors"

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
	sources, err := c.source.Discover(ctx)
	if err != nil {
		return nil, selectionError(err)
	}
	var candidate *activeSelection
	matches := 0
	for _, source := range sources {
		bundle, err := c.source.Load(ctx, source.Ref)
		if err != nil {
			return nil, selectionError(err)
		}
		for i, p := range bundle.Profiles {
			if p.ID != nil && *p.ID == selector || !idOnly && p.Name != nil && *p.Name == selector {
				matches++
				candidate = &activeSelection{selection: profilesource.Selection{Source: source.Ref, ProfileIndex: i + 1}, profile: p}
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
	var failure *profilesource.Error
	if errors.As(err, &failure) && failure.Code == profilesource.ERR_PROFILE_NOT_FOUND {
		return NewError(failure.Code)
	}
	return NewError(profilesource.ERR_PROFILE_STORAGE)
}
