package control

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/profilesource"
)

func localStateSelection(settings config.Profile) profilesource.LocalStateSelection {
	return profilesource.LocalStateSelection{Policy: settings.Source, StorePath: settings.LocalStorePath, Device: settings.LocalDevice, File: settings.LocalProfileFile}
}

func localActive(ref profilesource.LocalRef, candidate profilesource.LocalCandidate) *activeSelection {
	return &activeSelection{selection: profilesource.Selection{Source: profilesource.SourceRef{Local: ref}, ProfileIndex: 1}, profile: candidate.Bundle().Profiles[0]}
}

func localReadError(err error) *Error {
	var failure *profilesource.Error
	if errors.As(err, &failure) && failure.Code != profilesource.ERR_PROFILE_NOT_FOUND && failure.Code != profilesource.ERR_PROFILE_STORAGE {
		return selectionError(err)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return selectionError(err)
	}
	return NewError(profilesource.ERR_PROFILE_LOCAL_READ)
}

func (c *Controller) initializeProfile(ctx context.Context, settings config.Profile) {
	var startWatch func()
	defer func() {
		if startWatch != nil {
			startWatch()
		}
	}()
	if settings.SelectedID != "" || settings.Source == "imported" {
		selected, p, err := ResolveConfiguredProfile(ctx, c.source, settings)
		if err != nil {
			c.selectionFailure = selectionError(err)
		} else {
			c.active = &activeSelection{selection: selected, profile: p}
		}
		return
	}
	if settings.LocalDevice != "" {
		if settings.LocalStorePath != "" {
			absolute, err := filepath.Abs(settings.LocalStorePath)
			if err != nil {
				c.localFailure = NewError(profilesource.ERR_PROFILE_LOCAL_READ)
			} else {
				settings.LocalStorePath = absolute
			}
		}
		if c.localFailure == nil {
			startWatch = c.initializeLocal(ctx, settings)
		}
	}
	if c.active != nil {
		return
	}
	if settings.Source == "local" {
		if c.localFailure == nil {
			c.selectionFailure = NewError(ERR_PROFILE_SOURCE_UNAVAILABLE)
		}
		return
	}
	active, err := resolveSavedImported(ctx, c.source, true)
	if err != nil {
		c.selectionFailure = selectionError(err)
		return
	}
	c.active = active
}

func (c *Controller) initializeLocal(ctx context.Context, settings config.Profile) func() {
	state := localStateSelection(settings)
	ref, err := profilesource.ResolveLocalRef(settings.LocalStorePath, settings.LocalDevice, settings.LocalProfileFile)
	var resolveFailure *profilesource.Error
	if errors.As(err, &resolveFailure) && resolveFailure.Code == profilesource.ERR_PROFILE_NOT_FOUND {
		if savedRef, _, restoreErr := profilesource.RestoreLocalState(state); restoreErr == nil {
			ref = savedRef
			err = nil
		}
	}
	var changes <-chan profilesource.SourceChange
	var candidate profilesource.LocalCandidate
	localCtx, cancel := context.WithCancel(ctx)
	if err == nil {
		source := profilesource.NewAzeronLocalSource(ref.Root)
		if settings.Watch {
			changes, err = source.Watch(localCtx, profilesource.SourceRef{Local: ref})
			if err != nil {
				c.localWatchFailure = NewError(profilesource.ERR_PROFILE_LOCAL_WATCH)
			}
		}
		if changes != nil {
			select {
			case change, ok := <-changes:
				if ok && change.Candidate != nil {
					candidate = *change.Candidate
					err = nil
				} else if ok && change.Failure != nil {
					err = change.Failure
					if change.Failure.Code == profilesource.ERR_PROFILE_LOCAL_WATCH {
						c.localWatchFailure = selectionError(err)
					}
				} else {
					err = &profilesource.Error{Code: profilesource.ERR_PROFILE_LOCAL_WATCH}
					c.localWatchFailure = selectionError(err)
				}
			case <-ctx.Done():
				err = ctx.Err()
			}
		}
		if changes == nil || c.localWatchFailure != nil {
			candidate, err = source.LoadCandidate(localCtx, profilesource.SourceRef{Local: ref})
		}
	}
	if err == nil {
		err = profilesource.SaveLocalState(localCtx, state, ref, candidate)
		if err == nil {
			c.active = localActive(ref, candidate)
		}
	}
	if err != nil {
		c.localFailure = localReadError(err)
		if savedRef, saved, restoreErr := profilesource.RestoreLocalState(state); restoreErr == nil {
			c.active = localActive(savedRef, saved)
		} else if !errors.Is(restoreErr, os.ErrNotExist) && c.localFailure.Code == profilesource.ERR_PROFILE_LOCAL_READ {
			c.localFailure = selectionError(restoreErr)
		}
	}
	if changes == nil {
		cancel()
		return nil
	}
	done := make(chan struct{})
	c.localEnabled = true
	c.localCancel, c.localDone = cancel, done
	return func() { go c.consumeLocal(localCtx, state, changes, done) }
}

func (c *Controller) consumeLocal(ctx context.Context, state profilesource.LocalStateSelection, changes <-chan profilesource.SourceChange, done chan<- struct{}) {
	defer close(done)
	for change := range changes {
		if ctx.Err() != nil {
			continue
		}
		c.mu.Lock()
		enabled := c.localEnabled
		c.mu.Unlock()
		if !enabled {
			continue
		}
		var failure *Error
		if change.Failure != nil {
			failure = selectionError(change.Failure)
		} else if change.Candidate != nil {
			if err := profilesource.SaveLocalState(ctx, state, change.Ref.Local, *change.Candidate); err != nil {
				failure = selectionError(err)
			}
		}
		c.mu.Lock()
		if c.localEnabled && ctx.Err() == nil {
			if failure != nil {
				if failure.Code == profilesource.ERR_PROFILE_LOCAL_WATCH {
					c.localWatchFailure = failure
				} else {
					c.localFailure = failure
				}
			} else if change.Candidate != nil {
				c.active = localActive(change.Ref.Local, *change.Candidate)
				c.generation++
				c.localFailure = nil
				c.selectionFailure = nil
			}
		}
		c.mu.Unlock()
	}
}
