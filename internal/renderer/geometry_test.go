package renderer

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/sh4869221b/azerlay/internal/layout"
)

func syntheticShapes(t *testing.T) layout.Definition {
	t.Helper()
	data, err := os.ReadFile("../layout/testdata/valid-shapes.json")
	if err != nil {
		t.Fatal(err)
	}
	definition, err := layout.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return definition
}

func TestGeometryGolden(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/geometry.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		OrderedIDs                   []string   `json:"ordered_ids"`
		GroupBounds                  [4]float64 `json:"group_bounds"`
		NaturalSize                  [2]float64 `json:"natural_size"`
		ViewOriginAt232x116          [2]float64 `json:"view_origin_at_232x116"`
		ViewOriginWithBandsAt200x138 [2]float64 `json:"view_origin_with_bands_at_200x138"`
		EmbeddedControlCount         int        `json:"embedded_control_count"`
	}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	definition := syntheticShapes(t)
	ordered := orderedControls(definition)
	var ids []string
	for _, control := range ordered {
		ids = append(ids, control.ID)
	}
	if !reflect.DeepEqual(ids, want.OrderedIDs) {
		t.Fatalf("ordered controls: got %v, want %v", ids, want.OrderedIDs)
	}
	for _, control := range ordered {
		if control.ID == "group" {
			bounds := boundsOfShape(control.Shape, shapeTransform{scaleX: 1, scaleY: 1})
			got := [4]float64{bounds.minX, bounds.minY, bounds.maxX, bounds.maxY}
			if got != want.GroupBounds {
				t.Fatalf("transformed group bounds: got %v, want %v", got, want.GroupBounds)
			}
		}
	}
	placement := fitGeometry(definition.ViewBox, Options{Scale: 1}, 232, 116, 0, 0)
	if [2]float64{placement.NaturalWidth, placement.NaturalHeight} != want.NaturalSize || [2]float64{placement.X, placement.Y} != want.ViewOriginAt232x116 || placement.Scale != 1 {
		t.Fatalf("uniform fit: %+v", placement)
	}
	withBands := fitGeometry(definition.ViewBox, Options{Scale: 1}, 200, 138, 12, 10)
	if [2]float64{withBands.X, withBands.Y} != want.ViewOriginWithBandsAt200x138 || withBands.Scale != 1 {
		t.Fatalf("band layout: %+v", withBands)
	}
	embedded, err := layout.LoadEmbedded("cyborg-ii", "left")
	if err != nil {
		t.Fatal(err)
	}
	if len(embedded.Controls) != want.EmbeddedControlCount {
		t.Fatalf("embedded controls: %d", len(embedded.Controls))
	}
}
