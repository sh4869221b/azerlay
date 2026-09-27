package renderer

import (
	"math"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/profile"
)

func drawGeometry(cr *cairo.Context, snapshot *OverlaySnapshot, options Options, width, height, titleBand, statusBand float64) {
	placement := fitGeometry(snapshot.definition.ViewBox, options, width, height, titleBand, statusBand)
	if placement.Scale == 0 {
		return
	}
	cr.Save()
	defer cr.Restore()
	cr.Rectangle(0, 0, width, height)
	cr.Clip()
	cr.SetOperator(cairo.OperatorClear)
	cr.Paint()
	cr.SetOperator(cairo.OperatorOver)
	cr.PushGroup()

	view := snapshot.definition.ViewBox
	frameX := (width - (view.Width+2*geometryPadding)*placement.Scale) / 2
	frameY := (height - (view.Height+2*geometryPadding+titleBand+statusBand)*placement.Scale) / 2
	cr.Translate(frameX, frameY)
	cr.Scale(placement.Scale, placement.Scale)
	colors := themeFor(options)
	colors.background.set(cr)
	cr.Rectangle(0, 0, view.Width+2*geometryPadding, view.Height+2*geometryPadding+titleBand+statusBand)
	cr.Fill()
	cr.Translate(geometryPadding-view.X, geometryPadding+titleBand-view.Y)

	for _, shape := range snapshot.definition.Decorations {
		drawShape(cr, shape, colors.idle, colors.outline, 1, false)
	}
	for _, region := range orderedControls(snapshot.definition) {
		drawControl(cr, region, snapshot.content.Controls[region.ID], colors, options)
	}
	cr.PopGroupToSource()
	cr.PaintWithAlpha(options.Opacity)
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

func drawControl(cr *cairo.Context, region layout.Control, control Control, colors palette, options Options) {
	fill := colors.idle
	static := staticKind(control)
	if control.Known && control.Down {
		fill = colors.pressed
	} else if static == profile.BindingUnbound && options.ShowUnbound {
		fill = colors.unbound
	} else if static == profile.BindingUnknown {
		fill = colors.unknown
	}
	width := colors.strokeWidth
	if control.Known && control.Down {
		width++
	}
	physical := region.ID != "stick.main"
	drawShape(cr, region.Shape, fill, colors.outline, width, physical && !control.Known)
	bounds := boundsOfShape(region.Shape, shapeTransform{scaleX: 1, scaleY: 1})
	if physical {
		drawPhysicalMark(cr, bounds, control, colors)
	}
	markX := bounds.maxX - 6
	markY := bounds.minY + 6
	if options.ShowAmbiguous && hasAmbiguous(control) {
		drawLetterA(cr, markX, markY, colors.statusWarning)
		markX -= 8
	}
	if static == profile.BindingUnbound && options.ShowUnbound {
		colors.primary.set(cr)
		cr.SetLineWidth(1.5)
		cr.MoveTo(markX-2, markY)
		cr.LineTo(markX+2, markY)
		cr.Stroke()
	} else if static == profile.BindingUnknown {
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

func drawPhysicalMark(cr *cairo.Context, bounds shapeBounds, control Control, colors palette) {
	x, y := bounds.minX+6, bounds.minY+6
	if !control.Known {
		drawQuestionMark(cr, x, y, colors.primary)
		return
	}
	colors.primary.set(cr)
	cr.SetLineWidth(1.5)
	cr.NewPath()
	cr.Arc(x, y, 2.4, 0, 2*math.Pi)
	if control.Down {
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
