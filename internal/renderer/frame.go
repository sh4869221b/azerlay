package renderer

import (
	"math"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/pangocairo"
	"github.com/sh4869221b/azerlay/internal/layout"
)

type preparedControl struct {
	region layout.Control
	style  controlStyle
	box    shapeBounds
	lines  []preparedLine
}

type Frame struct {
	snapshot              *OverlaySnapshot
	options               Options
	width, height         float64
	titleBand, statusBand float64
	naturalTitleBand      float64
	placement             geometryPlacement
	colors                palette
	controls              []preparedControl
	title, status         *preparedLine
}

func (frame *Frame) NaturalSize() (float64, float64) {
	if frame == nil {
		return 0, 0
	}
	view := frame.snapshot.definition.ViewBox
	return (view.Width + 2*geometryPadding) * frame.options.Scale,
		(view.Height + 2*geometryPadding + frame.naturalTitleBand + frame.statusBand) * frame.options.Scale
}

// Prepare owns text measurement and must run on the drawing area's owner thread.
func Prepare(snapshot *OverlaySnapshot, options Options, width, height float64) *Frame {
	if snapshot == nil || width <= 0 || height <= 0 {
		return nil
	}
	surface := cairo.CreateImageSurface(cairo.FormatARGB32, 1, 1)
	defer surface.Close()
	context := cairo.Create(surface)
	defer context.Close()
	frame := &Frame{snapshot: snapshot, options: options, width: width, height: height, colors: themeFor(options)}
	view := snapshot.definition.ViewBox
	bandWidth := view.Width - 8
	if options.ShowStatus && snapshot.content.Status != StatusNone {
		message := snapshot.content.Status.message()
		if message != "" {
			ink := frame.colors.statusWarning
			if snapshot.content.Status == StatusReloadFailed {
				ink = frame.colors.statusError
			}
			line := prepareLine(context, textSpec{text: message, size: 12}, options, bandWidth-14, ink)
			frame.statusBand = line.height + 8
			line.x = view.X + 18
			line.y = view.Y + view.Height + 4
			frame.status = &line
		}
	}
	if options.ShowProfileName && snapshot.content.ProfileName != "" {
		line := prepareLine(context, textSpec{text: snapshot.content.ProfileName, size: 14}, options, bandWidth, frame.colors.primary)
		frame.titleBand = line.height + 8
		frame.naturalTitleBand = frame.titleBand
		line.x = view.X + 4
		line.y = view.Y - frame.titleBand + 4
		frame.title = &line
	}
	if frame.status != nil && frame.title != nil && height < frame.titleBand+frame.statusBand+2*geometryPadding {
		frame.title = nil
		frame.titleBand = 0
	}
	frame.placement = fitGeometry(view, options, width, height, frame.titleBand, frame.statusBand)
	if frame.placement.Scale == 0 {
		return nil
	}
	for _, region := range orderedControls(snapshot.definition) {
		control := snapshot.content.Controls[region.ID]
		item := preparedControl{region: region, style: prepareControlStyle(region, control, frame.colors, options)}
		item.lines, item.box = prepareControlLines(context, region, control, item.style, options, frame.colors)
		if len(item.lines) > 0 && item.lines[0].height*frame.placement.Scale < 1 {
			item.lines = nil
		}
		frame.controls = append(frame.controls, item)
	}
	return frame
}

func prepareControlLines(cr *cairo.Context, region layout.Control, control Control, style controlStyle, options Options, colors palette) ([]preparedLine, shapeBounds) {
	if region.Shape.Type == "line" || region.Shape.Type == "path" {
		return nil, shapeBounds{}
	}
	b := style.bounds
	left, right := b.minX+6, b.maxX-6
	top, bottom := b.minY+14, b.maxY-6
	availableWidth := 2 * math.Min(region.LabelAnchor.X-left, right-region.LabelAnchor.X)
	if availableWidth < 4 || bottom-top < 1 {
		return nil, shapeBounds{}
	}
	box := shapeBounds{region.LabelAnchor.X - availableWidth/2, top, region.LabelAnchor.X + availableWidth/2, bottom}
	var lines []preparedLine
	var totalHeight float64
	for _, spec := range controlText(control, options) {
		ink := colors.primary
		if spec.secondary {
			ink = colors.secondary
		}
		line := prepareLine(cr, spec, options, availableWidth, ink)
		if line.height <= 0 || totalHeight+line.height > bottom-top {
			break
		}
		totalHeight += line.height
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return nil, box
	}
	y := math.Max(top, math.Min(region.LabelAnchor.Y-totalHeight/2, bottom-totalHeight))
	for i := range lines {
		lines[i].x = box.minX
		lines[i].y = y
		y += lines[i].height
	}
	return lines, box
}

// Draw uses only geometry and text layouts prepared on the same owner thread.
func (frame *Frame) Draw(cr *cairo.Context) {
	if frame == nil {
		return
	}
	cr.Save()
	defer cr.Restore()
	cr.Rectangle(0, 0, frame.width, frame.height)
	cr.Clip()
	cr.SetOperator(cairo.OperatorClear)
	cr.Paint()
	cr.SetOperator(cairo.OperatorOver)
	cr.PushGroup()
	drawGeometryLayer(cr, frame)
	for _, item := range frame.controls {
		if len(item.lines) == 0 {
			continue
		}
		cr.Save()
		cr.NewPath()
		appendShapeClip(cr, item.region.Shape)
		cr.Clip()
		cr.Rectangle(item.box.minX, item.box.minY, item.box.maxX-item.box.minX, item.box.maxY-item.box.minY)
		cr.Clip()
		for _, line := range item.lines {
			line.ink.set(cr)
			cr.MoveTo(line.x, line.y)
			pangocairo.ShowLayout(cr, line.layout)
		}
		cr.Restore()
	}
	view := frame.snapshot.definition.ViewBox
	if frame.title != nil {
		cr.Save()
		cr.Rectangle(view.X, view.Y-frame.titleBand, view.Width, frame.titleBand)
		cr.Clip()
		frame.title.ink.set(cr)
		cr.MoveTo(frame.title.x, frame.title.y)
		pangocairo.ShowLayout(cr, frame.title.layout)
		cr.Restore()
	}
	if frame.status != nil {
		cr.Save()
		cr.Rectangle(view.X, view.Y+view.Height, view.Width, frame.statusBand+geometryPadding)
		cr.Clip()
		frame.status.ink.set(cr)
		x, y := view.X+9, view.Y+view.Height+4+frame.status.height/2
		cr.SetLineWidth(1.5)
		cr.MoveTo(x, y-4)
		cr.LineTo(x, y+1)
		cr.Stroke()
		cr.NewPath()
		cr.Arc(x, y+4, .8, 0, 2*math.Pi)
		cr.Fill()
		cr.MoveTo(frame.status.x, frame.status.y)
		pangocairo.ShowLayout(cr, frame.status.layout)
		cr.Restore()
	}
	cr.PopGroupToSource()
	cr.PaintWithAlpha(frame.options.Opacity)
}

func appendShapeClip(cr *cairo.Context, shape layout.Shape) {
	if shape.Type == "group" {
		cr.Save()
		cr.Translate(shape.Transform.TranslateX, shape.Transform.TranslateY)
		cr.Scale(shape.Transform.ScaleX, shape.Transform.ScaleY)
		for _, child := range shape.Children {
			appendShapeClip(cr, child)
		}
		cr.Restore()
		return
	}
	shapePath(cr, shape)
}
