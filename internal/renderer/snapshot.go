package renderer

import (
	"slices"

	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/profile"
)

type Status string

const (
	StatusNone           Status = ""
	StatusDisconnected   Status = "disconnected"
	StatusProfileMissing Status = "profile-missing"
	StatusReloadFailed   Status = "reload-failed"
)

func (s Status) message() string {
	switch s {
	case StatusDisconnected:
		return "Device disconnected"
	case StatusProfileMissing:
		return "Profile missing"
	case StatusReloadFailed:
		return "Reload failed"
	default:
		return ""
	}
}

// Assignment describes configured behavior, never trigger firing or progress.
type Assignment struct {
	Trigger        profile.TriggerKind
	Kind           profile.BindingKind
	Label          string
	BindingDisplay string
	Ambiguous      bool
}

type Control struct {
	Known       bool
	Down        bool
	Assignments []Assignment
}

type Content struct {
	ProfileName string
	Controls    map[string]Control
	Status      Status
}

type OverlaySnapshot struct {
	definition layout.Definition
	content    Content
}

func NewSnapshot(definition layout.Definition, content Content) *OverlaySnapshot {
	owned := definition
	owned.Controls = slices.Clone(definition.Controls)
	for i := range owned.Controls {
		owned.Controls[i].Shape = cloneShape(definition.Controls[i].Shape)
	}
	owned.Decorations = slices.Clone(definition.Decorations)
	for i, shape := range definition.Decorations {
		owned.Decorations[i] = cloneShape(shape)
	}
	if revision := definition.Applicability.HardwareRevision; revision != nil {
		copy := *revision
		owned.Applicability.HardwareRevision = &copy
	}

	values := Content{ProfileName: content.ProfileName, Status: content.Status, Controls: make(map[string]Control, len(owned.Controls))}
	for _, region := range owned.Controls {
		control := content.Controls[region.ID]
		if len(control.Assignments) == 0 {
			control.Assignments = []Assignment{{Trigger: profile.TriggerUnknown, Kind: profile.BindingUnknown}}
		}
		if region.ID == "stick.main" {
			control.Known = false
		}
		if !control.Known {
			control.Down = false
		}
		control.Assignments = slices.Clone(control.Assignments)
		values.Controls[region.ID] = control
	}
	return &OverlaySnapshot{definition: owned, content: values}
}

func cloneShape(source layout.Shape) layout.Shape {
	owned := source
	owned.Points = slices.Clone(source.Points)
	owned.Commands = slices.Clone(source.Commands)
	for i := range owned.Commands {
		owned.Commands[i].Points = slices.Clone(source.Commands[i].Points)
	}
	owned.Children = slices.Clone(source.Children)
	for i, child := range source.Children {
		owned.Children[i] = cloneShape(child)
	}
	return owned
}
