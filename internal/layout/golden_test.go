package layout

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"slices"
	"testing"
)

func TestGoldenLayoutFixtures(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		code ErrorCode
		path string
	}{
		{"valid-minimal.json", "", ""},
		{"invalid-json.json", ERR_LAYOUT_JSON, "$"},
		{"invalid-schema.json", ERR_LAYOUT_SCHEMA, "schema_version"},
		{"invalid-viewbox.json", ERR_LAYOUT_INVALID, "view_box.width"},
		{"invalid-missing-id.json", ERR_LAYOUT_SCHEMA, "controls[0].id"},
		{"invalid-duplicate-id.json", ERR_LAYOUT_INVALID, "controls[1].id"},
		{"invalid-shape.json", ERR_LAYOUT_INVALID, "controls[0].shape.radius"},
		{"invalid-anchor.json", ERR_LAYOUT_INVALID, "controls[0].label_anchor"},
		{"invalid-label.json", ERR_LAYOUT_SCHEMA, "controls[0]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile("testdata/" + tc.name)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Parse(data)
			if tc.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				if len(got.Controls) != 1 || got.Controls[0].ID != "grid.c1.r1" || got.Controls[0].PinOne != 255 || got.Controls[0].PinTwo != 4 {
					t.Fatalf("valid fixture changed position or literal pins: %+v", got.Controls)
				}
				return
			}
			var layoutErr *Error
			if !errors.As(err, &layoutErr) || layoutErr.Code != tc.code || layoutErr.Path != tc.path {
				t.Fatalf("want %s at %s, got %v", tc.code, tc.path, err)
			}
		})
	}
}

func TestGoldenLayoutReorderedControls(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/valid-shapes.json")
	if err != nil {
		t.Fatal(err)
	}
	first, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	var controls []json.RawMessage
	if err := json.Unmarshal(document["controls"], &controls); err != nil {
		t.Fatal(err)
	}
	slices.Reverse(controls)
	document["controls"], err = json.Marshal(controls)
	if err != nil {
		t.Fatal(err)
	}
	reorderedData, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	reordered, err := Parse(reorderedData)
	if err != nil {
		t.Fatal(err)
	}

	byPosition := make(map[string]Control, len(first.Controls))
	for _, control := range first.Controls {
		byPosition[control.ID] = control
	}
	if len(byPosition) != 7 || len(reordered.Controls) != len(byPosition) || byPosition["rect"].LabelAnchor != (Point{10, 10}) || byPosition["circle"].LabelAnchor != (Point{60, 10}) {
		t.Fatalf("synthetic fixture positions changed: %+v", byPosition)
	}
	for _, control := range reordered.Controls {
		if !reflect.DeepEqual(control, byPosition[control.ID]) {
			t.Errorf("position %s changed after reorder: %+v", control.ID, control)
		}
	}
}

func TestGoldenLayoutEmbeddedCenters(t *testing.T) {
	t.Parallel()

	definition, err := LoadEmbedded("cyborg-ii", "left")
	if err != nil {
		t.Fatal(err)
	}
	if len(definition.Controls) != 31 {
		t.Fatalf("want 31 regions, got %d", len(definition.Controls))
	}
	excluded := map[int]bool{21: true, 25: true, 26: true, 27: true, 32: true, 33: true, 34: true, 35: true, 39: true, 40: true, 42: true, 43: true}
	for _, control := range definition.Controls {
		if excluded[control.SourceInputID] {
			t.Errorf("excluded input %d has geometry", control.SourceInputID)
		}
		shape := control.Shape
		if shape.Type != "rounded_rect" || control.LabelAnchor != (Point{shape.X + shape.Width/2, shape.Y + shape.Height/2}) {
			t.Errorf("%s anchor is not its shape center", control.ID)
		}
		for _, other := range definition.Controls {
			if other.ID == control.ID {
				continue
			}
			if (bounds{other.Shape.X, other.Shape.Y, other.Shape.X + other.Shape.Width, other.Shape.Y + other.Shape.Height}).contains(control.LabelAnchor) {
				t.Errorf("%s anchor lies in unrelated region %s", control.ID, other.ID)
			}
		}
	}

	_, err = LoadEmbedded("cyborg-ii", "right")
	var layoutErr *Error
	if !errors.As(err, &layoutErr) || layoutErr.Code != ERR_LAYOUT_UNSUPPORTED || layoutErr.Path != "hand" {
		t.Fatalf("right-hand asset must be unsupported, got %v", err)
	}
}
