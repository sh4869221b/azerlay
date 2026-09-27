package renderer

import (
	"math"
	"slices"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/sh4869221b/azerlay/internal/layout"
)

const geometryPadding = 8.0

type geometryPlacement struct {
	X, Y          float64
	Scale         float64
	NaturalWidth  float64
	NaturalHeight float64
}

func fitGeometry(view layout.Rect, options Options, width, height, titleBand, statusBand float64) geometryPlacement {
	if width <= 0 || height <= 0 || view.Width <= 0 || view.Height <= 0 || options.Scale <= 0 {
		return geometryPlacement{}
	}
	naturalWidth := view.Width + 2*geometryPadding
	naturalHeight := view.Height + 2*geometryPadding + titleBand + statusBand
	requestedWidth := naturalWidth * options.Scale
	requestedHeight := naturalHeight * options.Scale
	factor := options.Scale * math.Min(width/requestedWidth, height/requestedHeight)
	return geometryPlacement{
		X:             (width-naturalWidth*factor)/2 + (geometryPadding-view.X)*factor,
		Y:             (height-naturalHeight*factor)/2 + (geometryPadding+titleBand-view.Y)*factor,
		Scale:         factor,
		NaturalWidth:  requestedWidth,
		NaturalHeight: requestedHeight,
	}
}

func orderedControls(definition layout.Definition) []layout.Control {
	controls := slices.Clone(definition.Controls)
	slices.SortFunc(controls, func(a, b layout.Control) int {
		if a.ZIndex < b.ZIndex {
			return -1
		}
		if a.ZIndex > b.ZIndex {
			return 1
		}
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return controls
}

func shapePath(cr *cairo.Context, shape layout.Shape) {
	switch shape.Type {
	case "rounded_rect":
		r := shape.Radius
		x, y, w, h := shape.X, shape.Y, shape.Width, shape.Height
		if r == 0 {
			cr.Rectangle(x, y, w, h)
			return
		}
		cr.NewSubPath()
		cr.Arc(x+w-r, y+r, r, -math.Pi/2, 0)
		cr.Arc(x+w-r, y+h-r, r, 0, math.Pi/2)
		cr.Arc(x+r, y+h-r, r, math.Pi/2, math.Pi)
		cr.Arc(x+r, y+r, r, math.Pi, 3*math.Pi/2)
		cr.ClosePath()
	case "polygon":
		cr.MoveTo(shape.Points[0].X, shape.Points[0].Y)
		for _, point := range shape.Points[1:] {
			cr.LineTo(point.X, point.Y)
		}
		cr.ClosePath()
	case "circle":
		cr.NewSubPath()
		cr.Arc(shape.CX, shape.CY, shape.Radius, 0, 2*math.Pi)
		cr.ClosePath()
	case "ellipse":
		cr.Save()
		cr.Translate(shape.CX, shape.CY)
		cr.Scale(shape.RX, shape.RY)
		cr.NewSubPath()
		cr.Arc(0, 0, 1, 0, 2*math.Pi)
		cr.ClosePath()
		cr.Restore()
	case "line":
		cr.MoveTo(shape.From.X, shape.From.Y)
		cr.LineTo(shape.To.X, shape.To.Y)
	case "path":
		for _, command := range shape.Commands {
			switch command.Op {
			case "M":
				cr.MoveTo(command.Points[0].X, command.Points[0].Y)
			case "L":
				cr.LineTo(command.Points[0].X, command.Points[0].Y)
			case "C":
				cr.CurveTo(command.Points[0].X, command.Points[0].Y, command.Points[1].X, command.Points[1].Y, command.Points[2].X, command.Points[2].Y)
			case "Z":
				cr.ClosePath()
			}
		}
	}
}

type shapeBounds struct{ minX, minY, maxX, maxY float64 }

type shapeTransform struct{ scaleX, scaleY, translateX, translateY float64 }

func (tr shapeTransform) point(point layout.Point) layout.Point {
	return layout.Point{X: tr.translateX + tr.scaleX*point.X, Y: tr.translateY + tr.scaleY*point.Y}
}

func (b shapeBounds) union(other shapeBounds) shapeBounds {
	return shapeBounds{math.Min(b.minX, other.minX), math.Min(b.minY, other.minY), math.Max(b.maxX, other.maxX), math.Max(b.maxY, other.maxY)}
}

func boundsOfShape(shape layout.Shape, tr shapeTransform) shapeBounds {
	var points []layout.Point
	switch shape.Type {
	case "rounded_rect":
		points = []layout.Point{{X: shape.X, Y: shape.Y}, {X: shape.X + shape.Width, Y: shape.Y + shape.Height}}
	case "polygon":
		points = shape.Points
	case "circle":
		points = []layout.Point{{X: shape.CX - shape.Radius, Y: shape.CY - shape.Radius}, {X: shape.CX + shape.Radius, Y: shape.CY + shape.Radius}}
	case "ellipse":
		points = []layout.Point{{X: shape.CX - shape.RX, Y: shape.CY - shape.RY}, {X: shape.CX + shape.RX, Y: shape.CY + shape.RY}}
	case "line":
		points = []layout.Point{shape.From, shape.To}
	case "path":
		for _, command := range shape.Commands {
			points = append(points, command.Points...)
		}
	case "group":
		child := shape.Transform
		childTr := shapeTransform{tr.scaleX * child.ScaleX, tr.scaleY * child.ScaleY, tr.translateX + tr.scaleX*child.TranslateX, tr.translateY + tr.scaleY*child.TranslateY}
		result := boundsOfShape(shape.Children[0], childTr)
		for _, item := range shape.Children[1:] {
			result = result.union(boundsOfShape(item, childTr))
		}
		return result
	}
	first := tr.point(points[0])
	result := shapeBounds{first.X, first.Y, first.X, first.Y}
	for _, point := range points[1:] {
		p := tr.point(point)
		result = result.union(shapeBounds{p.X, p.Y, p.X, p.Y})
	}
	return result
}
