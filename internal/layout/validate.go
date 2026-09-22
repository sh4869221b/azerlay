package layout

import (
	"fmt"
	"math"
	"strings"
)

func validateDefinition(definition Definition) error {
	view := definition.ViewBox
	for _, field := range []struct {
		name     string
		value    float64
		positive bool
	}{
		{"x", view.X, false}, {"y", view.Y, false},
		{"width", view.Width, true}, {"height", view.Height, true},
	} {
		if !finite(field.value) || (field.positive && field.value <= 0) {
			return invalid("view_box." + field.name)
		}
	}
	viewBounds := bounds{view.X, view.Y, view.X + view.Width, view.Y + view.Height}
	if !finite(viewBounds.maxX) || !finite(viewBounds.maxY) {
		return invalid("view_box")
	}

	ids := make(map[string]bool, len(definition.Controls))
	sources := make(map[int]bool, len(definition.Controls))
	for i, control := range definition.Controls {
		path := fmt.Sprintf("controls[%d]", i)
		if strings.TrimSpace(control.ID) == "" || ids[control.ID] {
			return invalid(path + ".id")
		}
		ids[control.ID] = true
		if strings.TrimSpace(control.Group) == "" {
			return invalid(path + ".group")
		}
		if control.SourceInputID <= 0 || sources[control.SourceInputID] {
			return invalid(path + ".source_input_id")
		}
		sources[control.SourceInputID] = true
		if control.PinOne < 0 {
			return invalid(path + ".pin_one")
		}
		if control.PinTwo < 0 {
			return invalid(path + ".pin_two")
		}
		if control.ZIndex < 0 {
			return invalid(path + ".z_index")
		}
		shapeBounds, err := validateShape(control.Shape, path+".shape", identityTransform(), viewBounds)
		if err != nil {
			return err
		}
		if !finitePoint(control.LabelAnchor) || !viewBounds.contains(control.LabelAnchor) || !shapeBounds.contains(control.LabelAnchor) {
			return invalid(path + ".label_anchor")
		}
	}
	for i, shape := range definition.Decorations {
		if _, err := validateShape(shape, fmt.Sprintf("decorations[%d]", i), identityTransform(), viewBounds); err != nil {
			return err
		}
	}
	return nil
}

func invalid(path string) error {
	return &Error{Code: ERR_LAYOUT_INVALID, Path: path}
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func finitePoint(point Point) bool {
	return finite(point.X) && finite(point.Y)
}
