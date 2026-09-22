package layout

import (
	"fmt"
	"math"
)

type bounds struct {
	minX, minY, maxX, maxY float64
}

func (b bounds) contains(p Point) bool {
	return p.X >= b.minX && p.X <= b.maxX && p.Y >= b.minY && p.Y <= b.maxY
}

func (b bounds) within(outer bounds) bool {
	return b.minX >= outer.minX && b.maxX <= outer.maxX && b.minY >= outer.minY && b.maxY <= outer.maxY
}

func (b bounds) union(other bounds) bounds {
	return bounds{
		math.Min(b.minX, other.minX), math.Min(b.minY, other.minY),
		math.Max(b.maxX, other.maxX), math.Max(b.maxY, other.maxY),
	}
}

type affine struct {
	sx, sy, tx, ty float64
}

func identityTransform() affine { return affine{sx: 1, sy: 1} }

func (a affine) point(p Point) Point {
	return Point{X: a.tx + a.sx*p.X, Y: a.ty + a.sy*p.Y}
}

func (a affine) rect(x, y, width, height float64) bounds {
	p := a.point(Point{x, y})
	q := a.point(Point{x + width, y + height})
	return bounds{math.Min(p.X, q.X), math.Min(p.Y, q.Y), math.Max(p.X, q.X), math.Max(p.Y, q.Y)}
}

func validBounds(b bounds) bool {
	return finite(b.minX) && finite(b.minY) && finite(b.maxX) && finite(b.maxY)
}

func validateShape(shape Shape, path string, transform affine, view bounds) (bounds, error) {
	var b bounds
	switch shape.Type {
	case "rounded_rect":
		for _, field := range []struct {
			name     string
			value    float64
			positive bool
		}{
			{"x", shape.X, false}, {"y", shape.Y, false},
			{"width", shape.Width, true}, {"height", shape.Height, true},
			{"radius", shape.Radius, false},
		} {
			if !finite(field.value) || (field.positive && field.value <= 0) {
				return bounds{}, invalid(path + "." + field.name)
			}
		}
		if shape.Radius < 0 || shape.Radius > math.Min(shape.Width, shape.Height)/2 {
			return bounds{}, invalid(path + ".radius")
		}
		b = transform.rect(shape.X, shape.Y, shape.Width, shape.Height)
	case "polygon":
		if len(shape.Points) < 3 {
			return bounds{}, invalid(path + ".points")
		}
		var area float64
		for i, p := range shape.Points {
			if !finitePoint(p) {
				return bounds{}, invalid(fmt.Sprintf("%s.points[%d]", path, i))
			}
			q := shape.Points[(i+1)%len(shape.Points)]
			area += p.X*q.Y - q.X*p.Y
		}
		if !finite(area) || area == 0 {
			return bounds{}, invalid(path + ".points")
		}
		b = pointsBounds(shape.Points, transform)
	case "circle":
		if !finite(shape.CX) {
			return bounds{}, invalid(path + ".cx")
		}
		if !finite(shape.CY) {
			return bounds{}, invalid(path + ".cy")
		}
		if !finite(shape.Radius) || shape.Radius <= 0 {
			return bounds{}, invalid(path + ".radius")
		}
		b = transform.rect(shape.CX-shape.Radius, shape.CY-shape.Radius, 2*shape.Radius, 2*shape.Radius)
	case "ellipse":
		if !finite(shape.CX) {
			return bounds{}, invalid(path + ".cx")
		}
		if !finite(shape.CY) {
			return bounds{}, invalid(path + ".cy")
		}
		if !finite(shape.RX) || shape.RX <= 0 {
			return bounds{}, invalid(path + ".rx")
		}
		if !finite(shape.RY) || shape.RY <= 0 {
			return bounds{}, invalid(path + ".ry")
		}
		b = transform.rect(shape.CX-shape.RX, shape.CY-shape.RY, 2*shape.RX, 2*shape.RY)
	case "line":
		if !finitePoint(shape.From) {
			return bounds{}, invalid(path + ".from")
		}
		if !finitePoint(shape.To) || shape.From == shape.To {
			return bounds{}, invalid(path + ".to")
		}
		b = pointsBounds([]Point{shape.From, shape.To}, transform)
	case "path":
		var err error
		b, err = validatePath(shape.Commands, path+".commands", transform)
		if err != nil {
			return bounds{}, err
		}
	case "group":
		tr := shape.Transform
		for _, field := range []struct {
			name  string
			value float64
			scale bool
		}{
			{"translate_x", tr.TranslateX, false}, {"translate_y", tr.TranslateY, false},
			{"scale_x", tr.ScaleX, true}, {"scale_y", tr.ScaleY, true},
		} {
			if !finite(field.value) || (field.scale && field.value <= 0) {
				return bounds{}, invalid(path + ".transform." + field.name)
			}
		}
		if len(shape.Children) == 0 {
			return bounds{}, invalid(path + ".children")
		}
		childTransform := affine{transform.sx * tr.ScaleX, transform.sy * tr.ScaleY,
			transform.tx + transform.sx*tr.TranslateX, transform.ty + transform.sy*tr.TranslateY}
		for i, child := range shape.Children {
			childBounds, err := validateShape(child, fmt.Sprintf("%s.children[%d]", path, i), childTransform, view)
			if err != nil {
				return bounds{}, err
			}
			if i == 0 {
				b = childBounds
			} else {
				b = b.union(childBounds)
			}
		}
		return b, nil
	default:
		return bounds{}, invalid(path + ".type")
	}
	if !validBounds(b) || !b.within(view) {
		return bounds{}, invalid(path)
	}
	return b, nil
}

func pointsBounds(points []Point, transform affine) bounds {
	first := transform.point(points[0])
	b := bounds{first.X, first.Y, first.X, first.Y}
	for _, p := range points[1:] {
		p = transform.point(p)
		b = b.union(bounds{p.X, p.Y, p.X, p.Y})
	}
	return b
}

func validatePath(commands []PathCommand, path string, transform affine) (bounds, error) {
	if len(commands) == 0 {
		return bounds{}, invalid(path)
	}
	var b bounds
	var current Point
	var subpathStart Point
	haveBounds := false
	open, drawn := false, false
	for i, command := range commands {
		commandPath := fmt.Sprintf("%s[%d]", path, i)
		want := 0
		switch command.Op {
		case "M":
			if i > 0 && open && !drawn {
				return bounds{}, invalid(commandPath + ".op")
			}
			want = 1
			open = true
			drawn = false
		case "L":
			want = 1
			if !open {
				return bounds{}, invalid(commandPath + ".op")
			}
			drawn = true
		case "C":
			want = 3
			if !open {
				return bounds{}, invalid(commandPath + ".op")
			}
			drawn = true
		case "Z":
			if !open || !drawn {
				return bounds{}, invalid(commandPath + ".op")
			}
			open = false
		default:
			return bounds{}, invalid(commandPath + ".op")
		}
		if i == 0 && command.Op != "M" {
			return bounds{}, invalid(commandPath + ".op")
		}
		if len(command.Points) != want {
			return bounds{}, invalid(commandPath + ".points")
		}
		for j, p := range command.Points {
			if !finitePoint(p) {
				return bounds{}, invalid(fmt.Sprintf("%s.points[%d]", commandPath, j))
			}
		}
		switch command.Op {
		case "M":
			current = transform.point(command.Points[0])
			subpathStart = current
		case "L", "Z":
			next := subpathStart
			if command.Op == "L" {
				next = transform.point(command.Points[0])
			}
			segment := pointsBounds([]Point{current, next}, identityTransform())
			if haveBounds {
				b = b.union(segment)
			} else {
				b = segment
				haveBounds = true
			}
			current = next
		case "C":
			segment := cubicBounds(current,
				transform.point(command.Points[0]),
				transform.point(command.Points[1]),
				transform.point(command.Points[2]))
			if haveBounds {
				b = b.union(segment)
			} else {
				b = segment
				haveBounds = true
			}
			current = transform.point(command.Points[2])
		}
	}
	if !drawn || !haveBounds {
		return bounds{}, invalid(path)
	}
	if b.minX == b.maxX && b.minY == b.maxY {
		return bounds{}, invalid(path)
	}
	return b, nil
}

func cubicBounds(p0, p1, p2, p3 Point) bounds {
	minX, maxX := cubicRange(p0.X, p1.X, p2.X, p3.X)
	minY, maxY := cubicRange(p0.Y, p1.Y, p2.Y, p3.Y)
	return bounds{minX, minY, maxX, maxY}
}

func cubicRange(p0, p1, p2, p3 float64) (float64, float64) {
	lo, hi := math.Min(p0, p3), math.Max(p0, p3)
	a := -p0 + 3*p1 - 3*p2 + p3
	b := 3*p0 - 6*p1 + 3*p2
	c := -3*p0 + 3*p1
	if !finite(a) || !finite(b) || !finite(c) {
		return math.NaN(), math.NaN()
	}
	var roots []float64
	if a == 0 {
		if b != 0 {
			roots = append(roots, -c/(2*b))
		}
	} else {
		discriminant := b*b - 3*a*c
		if !finite(discriminant) {
			return math.NaN(), math.NaN()
		}
		if discriminant >= 0 {
			root := math.Sqrt(discriminant)
			roots = append(roots, (-b-root)/(3*a), (-b+root)/(3*a))
		}
	}
	for _, t := range roots {
		if t <= 0 || t >= 1 {
			continue
		}
		u := 1 - t
		value := u*u*u*p0 + 3*u*u*t*p1 + 3*u*t*t*p2 + t*t*t*p3
		lo = math.Min(lo, value)
		hi = math.Max(hi, value)
	}
	return lo, hi
}
