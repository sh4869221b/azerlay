package layout

import (
	"errors"
	"math"
	"os"
	"strings"
	"testing"
)

func TestValidateLayout(t *testing.T) {
	t.Parallel()

	minimal, err := os.ReadFile("testdata/valid-minimal.json")
	if err != nil {
		t.Fatal(err)
	}
	shapes, err := os.ReadFile("testdata/valid-shapes.json")
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		data []byte
		want int
	}{
		{"one control and literal 255", minimal, 1},
		{"all primitives, boundary, repeated z index, transformed group", shapes, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.data)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Controls) != tc.want {
				t.Fatalf("controls: got %d, want %d", len(got.Controls), tc.want)
			}
			if got.Controls[0].PinOne != 255 {
				t.Fatalf("pin 255 changed: %d", got.Controls[0].PinOne)
			}
		})
	}

	for _, tc := range []struct {
		name, controlX, anchorX string
		valid                   bool
	}{
		{"cubic control point outside view but curve inside", "120", "50", true},
		{"anchor outside actual cubic bounds", "100", "90", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := strings.Replace(string(minimal), `"width": 694, "height": 640`, `"width": 100, "height": 100`, 1)
			data = strings.Replace(data, `"shape": {"type": "rounded_rect", "x": 10, "y": 20, "width": 70, "height": 100, "radius": 6}`, `"shape": {"type": "path", "commands": [{"op": "M", "points": [{"x": 20, "y": 20}]}, {"op": "C", "points": [{"x": `+tc.controlX+`, "y": 20}, {"x": `+tc.controlX+`, "y": 80}, {"x": 20, "y": 80}]}]}`, 1)
			data = strings.Replace(data, `"label_anchor": {"x": 45, "y": 70}`, `"label_anchor": {"x": `+tc.anchorX+`, "y": 50}`, 1)
			_, err := Parse([]byte(data))
			if tc.valid {
				if err != nil {
					t.Fatalf("curve fits view and anchor: %v", err)
				}
				return
			}
			assertLayoutError(t, err, ERR_LAYOUT_INVALID, "controls[0].label_anchor")
		})
	}

	invalid := []struct {
		name, source, old, replacement string
		path                           string
	}{
		{"zero view width", "minimal", `"width": 694`, `"width": 0`, "view_box.width"},
		{"negative view height", "minimal", `"height": 640`, `"height": -1`, "view_box.height"},
		{"overflowing view end", "minimal", `"x": 0, "y": 0, "width": 694`, `"x": 1e308, "y": 0, "width": 1e308`, "view_box"},
		{"zero shape width", "minimal", `"width": 70`, `"width": 0`, "controls[0].shape.width"},
		{"negative radius", "minimal", `"radius": 6`, `"radius": -1`, "controls[0].shape.radius"},
		{"radius above half", "minimal", `"radius": 6`, `"radius": 36`, "controls[0].shape.radius"},
		{"shape outside view", "minimal", `"x": 10, "y": 20`, `"x": 690, "y": 20`, "controls[0].shape"},
		{"anchor outside shape", "minimal", `"label_anchor": {"x": 45`, `"label_anchor": {"x": 100`, "controls[0].label_anchor"},
		{"anchor outside view", "minimal", `"label_anchor": {"x": 45`, `"label_anchor": {"x": 700`, "controls[0].label_anchor"},
		{"zero source input", "minimal", `"source_input_id": 1`, `"source_input_id": 0`, "controls[0].source_input_id"},
		{"negative first pin", "minimal", `"pin_one": 255`, `"pin_one": -1`, "controls[0].pin_one"},
		{"negative second pin", "minimal", `"pin_two": 4`, `"pin_two": -1`, "controls[0].pin_two"},
		{"negative z index", "minimal", `"z_index": 0`, `"z_index": -1`, "controls[0].z_index"},
		{"empty control id", "minimal", `"id": "grid.c1.r1"`, `"id": ""`, "controls[0].id"},
		{"empty control group", "minimal", `"group": "grid"`, `"group": ""`, "controls[0].group"},
		{"degenerate polygon", "shapes", `{"x": 35, "y": 10}`, `{"x": 35, "y": 0}`, "controls[1].shape.points"},
		{"short polygon", "shapes", `, {"x": 35, "y": 10}`, ``, "controls[1].shape.points"},
		{"zero circle radius", "shapes", `"cx": 60, "cy": 10, "radius": 10`, `"cx": 60, "cy": 10, "radius": 0`, "controls[2].shape.radius"},
		{"negative ellipse radius", "shapes", `"rx": 15`, `"rx": -1`, "controls[3].shape.rx"},
		{"degenerate line", "shapes", `"to": {"x": 20, "y": 40}`, `"to": {"x": 0, "y": 30}`, "controls[4].shape.to"},
		{"path must start at move", "shapes", `"op": "M"`, `"op": "L"`, "controls[5].shape.commands[0].op"},
		{"path cubic point count", "shapes", `{"x": 45, "y": 30}, {"x": 45, "y": 40}, {"x": 30, "y": 40}`, `{"x": 45, "y": 30}, {"x": 30, "y": 40}`, "controls[5].shape.commands[2].points"},
		{"path close point count", "shapes", `"op": "Z", "points": []`, `"op": "Z", "points": [{"x": 30, "y": 30}]`, "controls[5].shape.commands[3].points"},
		{"path draws after close", "shapes", `"op": "Z", "points": []`, `"op": "Z", "points": []}, {"op": "L", "points": [{"x": 40, "y": 40}]`, "controls[5].shape.commands[4].op"},
		{"path move without drawing", "shapes", `"op": "L", "points": [{"x": 40, "y": 30}]`, `"op": "M", "points": [{"x": 40, "y": 30}]`, "controls[5].shape.commands[1].op"},
		{"path unsupported command", "shapes", `"op": "C"`, `"op": "Q"`, "controls[5].shape.commands[2].op"},
		{"empty group", "shapes", `"children": [{"type": "rounded_rect", "x": 0, "y": 0, "width": 10, "height": 10, "radius": 0}, {"type": "circle", "cx": 10, "cy": 10, "radius": 5}]`, `"children": []`, "controls[6].shape.children"},
		{"zero group scale", "shapes", `"scale_x": 2`, `"scale_x": 0`, "controls[6].shape.transform.scale_x"},
		{"transformed child outside view", "shapes", `"translate_x": 70`, `"translate_x": 80`, "controls[6].shape.children[1]"},
		{"group anchor outside bounds", "shapes", `"label_anchor": {"x": 80, "y": 80}`, `"label_anchor": {"x": 60, "y": 80}`, "controls[6].label_anchor"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			base := minimal
			if tc.source == "shapes" {
				base = shapes
			}
			if !strings.Contains(string(base), tc.old) {
				t.Fatalf("test replacement absent: %s", tc.old)
			}
			data := []byte(strings.Replace(string(base), tc.old, tc.replacement, 1))
			_, err := Parse(data)
			assertLayoutError(t, err, ERR_LAYOUT_INVALID, tc.path)
		})
	}

	for _, tc := range []struct {
		name, scaleField, anchor, path string
	}{
		{"negative x scale", `"scale_x": 2`, `"label_anchor": {"x": 60, "y": 80}`, "controls[6].shape.transform.scale_x"},
		{"negative y scale", `"scale_y": 2`, `"label_anchor": {"x": 80, "y": 60}`, "controls[6].shape.transform.scale_y"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := strings.Replace(string(shapes), tc.scaleField, strings.Replace(tc.scaleField, `: 2`, `: -2`, 1), 1)
			data = strings.Replace(data, `"label_anchor": {"x": 80, "y": 80}`, tc.anchor, 1)
			_, err := Parse([]byte(data))
			assertLayoutError(t, err, ERR_LAYOUT_INVALID, tc.path)
		})
	}

	t.Run("duplicate ids and source ids", func(t *testing.T) {
		for _, tc := range []struct{ old, replacement, path string }{
			{`"id": "polygon"`, `"id": "rect"`, "controls[1].id"},
			{`"source_input_id": 2`, `"source_input_id": 1`, "controls[1].source_input_id"},
		} {
			_, err := Parse([]byte(strings.Replace(string(shapes), tc.old, tc.replacement, 1)))
			assertLayoutError(t, err, ERR_LAYOUT_INVALID, tc.path)
		}
	})

	t.Run("missing shape and anchor", func(t *testing.T) {
		for _, tc := range []struct{ old, path string }{
			{`"shape": {"type": "rounded_rect", "x": 10, "y": 20, "width": 70, "height": 100, "radius": 6},`, "controls[0].shape"},
			{`,
      "label_anchor": {"x": 45, "y": 70}`, "controls[0].label_anchor"},
		} {
			if !strings.Contains(string(minimal), tc.old) {
				t.Fatalf("test replacement absent: %s", tc.old)
			}
			_, err := Parse([]byte(strings.Replace(string(minimal), tc.old, "", 1)))
			assertLayoutError(t, err, ERR_LAYOUT_SCHEMA, tc.path)
		}
	})

	t.Run("nonfinite values in typed definition", func(t *testing.T) {
		definition, err := Parse(minimal)
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			name   string
			change func(*Definition)
			path   string
		}{
			{"view NaN", func(d *Definition) { d.ViewBox.X = math.NaN() }, "view_box.x"},
			{"shape infinity", func(d *Definition) { d.Controls[0].Shape.Width = math.Inf(1) }, "controls[0].shape.width"},
			{"transform infinity", func(d *Definition) {
				d.Controls[0].Shape = Shape{Type: "group", Transform: Transform{ScaleX: math.Inf(1), ScaleY: 1}, Children: []Shape{d.Controls[0].Shape}}
			}, "controls[0].shape.transform.scale_x"},
			{"anchor NaN", func(d *Definition) { d.Controls[0].LabelAnchor.X = math.NaN() }, "controls[0].label_anchor"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				copy := definition
				copy.Controls = append([]Control(nil), definition.Controls...)
				tc.change(&copy)
				assertLayoutError(t, validateDefinition(copy), ERR_LAYOUT_INVALID, tc.path)
			})
		}
	})
}

func assertLayoutError(t *testing.T, err error, code ErrorCode, path string) {
	t.Helper()
	var layoutErr *Error
	if !errors.As(err, &layoutErr) {
		t.Fatalf("want %s at %s, got %v", code, path, err)
	}
	if layoutErr.Code != code || layoutErr.Path != path {
		t.Fatalf("want %s at %s, got %s at %s", code, path, layoutErr.Code, layoutErr.Path)
	}
}
