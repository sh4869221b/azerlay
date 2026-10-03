package control

import (
	"slices"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profilesource"
)

// ProfileSnapshot is owned by the controller and immutable after publication.
// Consumers must not mutate its profile or any of its nested values. Opaque
// source data is omitted; a nil Profile represents an unavailable selection.
type ProfileSnapshot struct {
	Generation uint64
	Selection  profilesource.Selection
	Source     profile.SourceMetadata
	Profile    *profile.Profile
}

// ProfileSnapshot returns the current immutable pointer without copying input
// bindings on each raw observation.
func (c *Controller) ProfileSnapshot() *ProfileSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.profileSnapshot
}

// Changes coalesces profile and source-diagnostic changes for one consumer.
func (c *Controller) Changes() <-chan struct{} { return c.changes }

func (c *Controller) notifyChangedLocked() {
	select {
	case c.changes <- struct{}{}:
	default:
	}
}

// Called under mu, or during initialization before the watcher is started.
func (c *Controller) publishProfileLocked() {
	c.profileGeneration++
	next := &ProfileSnapshot{Generation: c.profileGeneration}
	if c.active != nil {
		next.Selection, next.Source = c.active.selection, c.active.source
		owned := cloneRenderProfile(c.active.profile)
		next.Profile = &owned
	}
	c.profileSnapshot = next
}

func cloneRenderValue[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneRenderProfile(value profile.Profile) profile.Profile {
	result := profile.Profile{ID: cloneRenderValue(value.ID), Name: cloneRenderValue(value.Name)}
	if value.Controls == nil {
		return result
	}
	result.Controls = make([]profile.ControlBinding, len(value.Controls))
	for i, control := range value.Controls {
		identity := control.SourceIdentity
		identity.InputID = cloneRenderValue(identity.InputID)
		identity.PinOne = cloneRenderValue(identity.PinOne)
		identity.PinTwo = cloneRenderValue(identity.PinTwo)
		out := profile.ControlBinding{Label: cloneRenderValue(control.Label), SourceIdentity: identity, Bindings: slices.Clone(control.Bindings)}
		for j := range out.Bindings {
			binding := &out.Bindings[j]
			binding.Actions = slices.Clone(binding.Actions)
			for k := range binding.Actions {
				binding.Actions[k].Modifiers = slices.Clone(binding.Actions[k].Modifiers)
			}
			binding.Turbo = cloneRenderValue(binding.Turbo)
			binding.Macro = cloneRenderValue(binding.Macro)
			if binding.Macro != nil {
				binding.Macro.Steps = slices.Clone(binding.Macro.Steps)
			}
			binding.Stick = cloneRenderValue(binding.Stick)
			binding.TriggerDelayMS = cloneRenderValue(binding.TriggerDelayMS)
			binding.TriggerIntervalMS = cloneRenderValue(binding.TriggerIntervalMS)
			binding.ReleaseBehavior = cloneRenderValue(binding.ReleaseBehavior)
			binding.Unknown = cloneRenderValue(binding.Unknown)
		}
		result.Controls[i] = out
	}
	return result
}
