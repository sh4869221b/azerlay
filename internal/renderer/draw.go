package renderer

import (
	"math"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/profile"
)

type physicalMark uint8

const (
	physicalNone physicalMark = iota
	physicalUnknown
	physicalReleased
	physicalPressed
)

type controlStyle struct {
	fill      color
	width     float64
	dashed    bool
	physical  physicalMark
	static    profile.BindingKind
	ambiguous bool
	bounds    shapeBounds
}

func prepareControlStyle(region layout.Control, control Control, colors palette, options Options) controlStyle {
	style := controlStyle{
		fill:      colors.idle,
		width:     colors.strokeWidth,
		static:    staticKind(control),
		ambiguous: options.ShowAmbiguous && hasAmbiguous(control),
		bounds:    boundsOfShape(region.Shape, shapeTransform{scaleX: 1, scaleY: 1}),
	}
	if region.ID != "stick.main" {
		switch {
		case !control.Known:
			style.physical = physicalUnknown
			style.dashed = true
		case control.Down:
			style.physical = physicalPressed
		case !control.Down:
			style.physical = physicalReleased
		}
	}
	if style.physical == physicalPressed {
		style.fill = colors.pressed
		style.width++
	} else if style.static == profile.BindingUnbound && options.ShowUnbound {
		style.fill = colors.unbound
	} else if style.static == profile.BindingUnknown {
		style.fill = colors.unknown
	}
	if style.static == profile.BindingUnbound && !options.ShowUnbound {
		style.static = ""
	}
	return style
}

func drawGeometryLayer(cr *cairo.Context, frame *Frame) {
	view := frame.snapshot.definition.ViewBox
	frameX := (frame.width - (view.Width+2*geometryPadding)*frame.placement.Scale) / 2
	frameY := (frame.height - (view.Height+2*geometryPadding+frame.titleBand+frame.statusBand)*frame.placement.Scale) / 2
	cr.Translate(frameX, frameY)
	cr.Scale(frame.placement.Scale, frame.placement.Scale)
	frame.colors.background.set(cr)
	cr.Rectangle(0, 0, view.Width+2*geometryPadding, view.Height+2*geometryPadding+frame.titleBand+frame.statusBand)
	cr.Fill()
	cr.Translate(geometryPadding-view.X, geometryPadding+frame.titleBand-view.Y)
	for _, shape := range frame.snapshot.definition.Decorations {
		drawShape(cr, shape, frame.colors.idle, frame.colors.outline, 1, false)
	}
}

func drawShape(cr *cairo.Context, shape layout.Shape, fill, outline color, width float64, dashed bool) {
	if shape.Type == "group" {
		cr.Save()
		cr.Translate(shape.Transform.TranslateX, shape.Transform.TranslateY)
		cr.Scale(shape.Transform.ScaleX, shape.Transform.ScaleY)
		for _, child := range shape.Children {
			drawShape(cr, child, fill, outline, width, dashed)
		}
		cr.Restore()
		return
	}
	cr.Save()
	cr.NewPath()
	shapePath(cr, shape)
	closed := shape.Type != "line" && (shape.Type != "path" || shape.Commands[len(shape.Commands)-1].Op == "Z")
	if closed {
		fill.set(cr)
		cr.FillPreserve()
	}
	outline.set(cr)
	cr.SetLineWidth(width)
	if dashed {
		cr.SetDash([]float64{3, 2}, 0)
	}
	cr.Stroke()
	cr.Restore()
}

func drawControlStyle(cr *cairo.Context, region layout.Control, style controlStyle, colors palette) {
	drawShape(cr, region.Shape, style.fill, colors.outline, style.width, style.dashed)
	if style.physical != physicalNone {
		drawPhysicalMark(cr, style.bounds, style.physical, colors)
	}
	markX := style.bounds.maxX - 6
	markY := style.bounds.minY + 6
	if style.ambiguous {
		drawLetterA(cr, markX, markY, colors.statusWarning)
		markX -= 8
	}
	if style.static == profile.BindingUnbound {
		colors.primary.set(cr)
		cr.SetLineWidth(1.5)
		cr.MoveTo(markX-2, markY)
		cr.LineTo(markX+2, markY)
		cr.Stroke()
	} else if style.static == profile.BindingUnknown {
		drawQuestionMark(cr, markX, markY, colors.primary)
	}
}

func staticKind(control Control) profile.BindingKind {
	if len(control.Assignments) == 0 {
		return profile.BindingUnknown
	}
	kind := control.Assignments[0].Kind
	if kind != profile.BindingUnbound && kind != profile.BindingUnknown {
		return ""
	}
	for _, assignment := range control.Assignments[1:] {
		if assignment.Kind != kind {
			return ""
		}
	}
	return kind
}

func hasAmbiguous(control Control) bool {
	for _, assignment := range control.Assignments {
		if assignment.Ambiguous {
			return true
		}
	}
	return false
}

func drawPhysicalMark(cr *cairo.Context, bounds shapeBounds, mark physicalMark, colors palette) {
	x, y := bounds.minX+6, bounds.minY+6
	if mark == physicalUnknown {
		drawQuestionMark(cr, x, y, colors.primary)
		return
	}
	colors.primary.set(cr)
	cr.SetLineWidth(1.5)
	cr.NewPath()
	cr.Arc(x, y, 2.4, 0, 2*math.Pi)
	if mark == physicalPressed {
		cr.Fill()
	} else {
		cr.Stroke()
	}
}

func drawQuestionMark(cr *cairo.Context, x, y float64, ink color) {
	cr.Save()
	ink.set(cr)
	cr.SetLineWidth(1.5)
	cr.NewPath()
	cr.Arc(x, y-1, 2.3, math.Pi, 2*math.Pi)
	cr.LineTo(x, y+2)
	cr.Stroke()
	cr.NewPath()
	cr.Arc(x, y+4, .8, 0, 2*math.Pi)
	cr.Fill()
	cr.Restore()
}

func drawLetterA(cr *cairo.Context, x, y float64, ink color) {
	cr.Save()
	ink.set(cr)
	cr.SetLineWidth(1.2)
	cr.MoveTo(x-3, y+3)
	cr.LineTo(x, y-4)
	cr.LineTo(x+3, y+3)
	cr.MoveTo(x-2, y+1)
	cr.LineTo(x+2, y+1)
	cr.Stroke()
	cr.Restore()
}
