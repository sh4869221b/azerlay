package renderer

import (
	"fmt"
	"slices"
	"strings"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotk4/pkg/pangocairo"
	"github.com/sh4869221b/azerlay/internal/profile"
)

type textSpec struct {
	text      string
	size      float64
	secondary bool
}

type preparedLine struct {
	layout     *pango.Layout
	text       string
	x, y       float64
	width      float64
	height     float64
	ellipsized bool
	ink        color
}

func triggerPriority(kind profile.TriggerKind) int {
	switch kind {
	case profile.TriggerSingle:
		return 0
	case profile.TriggerLong:
		return 1
	case profile.TriggerDouble:
		return 2
	default:
		return 3
	}
}

func triggerLabel(kind profile.TriggerKind) string {
	switch kind {
	case profile.TriggerSingle:
		return "SINGLE"
	case profile.TriggerLong:
		return "HOLD"
	case profile.TriggerDouble:
		return "DOUBLE"
	default:
		return "?"
	}
}

func controlText(control Control, options Options) []textSpec {
	if len(control.Assignments) == 0 || staticKind(control) == profile.BindingUnbound && !options.ShowUnbound {
		return nil
	}
	assignments := slices.Clone(control.Assignments)
	slices.SortStableFunc(assignments, func(a, b Assignment) int {
		return triggerPriority(a.Trigger) - triggerPriority(b.Trigger)
	})
	primary := assignments[0]
	label := primary.Label
	if label == "" && options.Mode == "compact" {
		label = primary.BindingDisplay
	}
	if label == "" && primary.BindingDisplay == "" {
		if primary.Kind == profile.BindingUnbound {
			label = "UNBOUND"
		} else {
			label = "UNKNOWN"
		}
	}
	var lines []textSpec
	if label != "" {
		lines = append(lines, textSpec{text: label, size: 14})
	}
	if options.Mode == "compact" {
		return lines
	}
	if primary.BindingDisplay != "" {
		lines = append(lines, textSpec{text: primary.BindingDisplay, size: 12, secondary: true})
	}
	if options.Mode != "detailed" {
		return lines
	}
	var metadata []string
	if primary.Trigger != profile.TriggerSingle {
		if primary.Trigger == profile.TriggerUnknown {
			metadata = append(metadata, "UNKNOWN")
		} else {
			metadata = append(metadata, triggerLabel(primary.Trigger))
		}
	}
	switch primary.Kind {
	case profile.BindingMacro:
		metadata = append(metadata, "MACRO")
	case profile.BindingUnbound:
		metadata = append(metadata, "UNBOUND")
	case profile.BindingUnknown:
		metadata = append(metadata, "UNKNOWN")
	}
	if primary.Ambiguous && options.ShowAmbiguous {
		metadata = append(metadata, "AMBIG")
	}
	if len(metadata) > 0 {
		lines = append(lines, textSpec{text: strings.Join(metadata, " · "), size: 10, secondary: true})
	}
	for _, assignment := range assignments[1:] {
		text := fmt.Sprintf("%s: %s", triggerLabel(assignment.Trigger), assignment.Label)
		if assignment.BindingDisplay != "" {
			if assignment.Label != "" {
				text += " · "
			}
			text += assignment.BindingDisplay
		}
		lines = append(lines, textSpec{text: text, size: 10, secondary: true})
	}
	return lines
}

func prepareLine(cr *cairo.Context, spec textSpec, options Options, width float64, ink color) preparedLine {
	layout := pangocairo.CreateLayout(cr)
	font := pango.FontDescriptionFromString("Sans")
	font.SetAbsoluteSize(spec.size * options.FontScale * pango.SCALE)
	layout.SetFontDescription(font)
	layout.SetSingleParagraphMode(true)
	layout.SetEllipsize(pango.EllipsizeEnd)
	layout.SetAlignment(pango.AlignCenter)
	layout.SetWidth(pango.UnitsFromDouble(width))
	text := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, spec.text)
	layout.SetText(text)
	_, logical := layout.PixelExtents()
	return preparedLine{layout: layout, text: text, width: width, height: float64(logical.Height()), ellipsized: layout.IsEllipsized(), ink: ink}
}
