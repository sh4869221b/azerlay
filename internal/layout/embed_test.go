package layout

import (
	"errors"
	"testing"
)

func TestLoadEmbeddedLeft(t *testing.T) {
	t.Parallel()

	type expectedControl struct {
		id, group           string
		source, pin1, pin2  int
		x, y, width, height float64
	}
	want := []expectedControl{
		{"grid.c1.r1", "grid", 4, 5, 255, 78, 108, 70, 100},
		{"grid.c1.r2", "grid", 3, 4, 255, 78, 216, 70, 100},
		{"grid.c1.r3", "grid", 2, 3, 255, 78, 324, 70, 100},
		{"grid.c1.r4", "grid", 1, 2, 255, 78, 432, 70, 100},
		{"grid.c1.r5", "grid", 37, 1, 255, 78, 540, 70, 100},
		{"grid.c2.r1", "grid", 8, 11, 255, 156, 108, 70, 100},
		{"grid.c2.r2", "grid", 7, 10, 255, 156, 216, 70, 100},
		{"grid.c2.r3", "grid", 6, 9, 255, 156, 324, 70, 100},
		{"grid.c2.r4", "grid", 5, 8, 255, 156, 432, 70, 100},
		{"grid.c2.r5", "grid", 38, 7, 255, 156, 540, 70, 100},
		{"grid.c3.r1", "grid", 12, 17, 255, 234, 108, 70, 100},
		{"grid.c3.r2", "grid", 11, 16, 255, 234, 216, 70, 100},
		{"grid.c3.r3", "grid", 10, 15, 255, 234, 324, 70, 100},
		{"grid.c3.r4", "grid", 9, 14, 255, 234, 432, 70, 100},
		{"grid.c3.r5", "grid", 13, 13, 255, 234, 540, 70, 100},
		{"grid.c4.r1", "grid", 17, 26, 255, 312, 108, 70, 100},
		{"grid.c4.r2", "grid", 16, 25, 255, 312, 216, 70, 100},
		{"grid.c4.r3", "grid", 15, 24, 255, 312, 324, 70, 100},
		{"grid.c4.r4", "grid", 14, 23, 255, 312, 432, 70, 100},
		{"grid.c4.r5", "grid", 18, 22, 255, 312, 540, 70, 100},
		{"grid.side-left", "grid", 36, 6, 255, 0, 324, 70, 100},
		{"grid.side-right", "grid", 19, 27, 255, 390, 324, 70, 100},
		{"cluster.top", "cluster", 28, 34, 255, 546, 0, 70, 100},
		{"cluster.left", "cluster", 29, 35, 255, 468, 108, 70, 100},
		{"cluster.center", "cluster", 22, 37, 255, 546, 108, 70, 100},
		{"cluster.right", "cluster", 31, 33, 255, 624, 108, 70, 100},
		{"cluster.bottom", "cluster", 30, 36, 255, 546, 216, 70, 100},
		{"stick.main", "stick", 24, 31, 30, 468, 324, 148, 208},
		{"stick.right-upper", "stick", 41, 20, 255, 624, 324, 70, 100},
		{"stick.right-lower", "stick", 20, 19, 255, 624, 432, 70, 100},
		{"stick.below", "stick", 23, 32, 255, 468, 540, 70, 100},
	}

	got, err := LoadEmbedded("cyborg-ii", "left")
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != 1 || got.Model != "cyborg-ii" || got.Hand != "left" || got.ViewBox != (Rect{0, 0, 694, 640}) {
		t.Fatalf("layout scope or view box: %+v", got)
	}
	if got.Applicability.SoftwareRelease != "2.0.2" || got.Applicability.DisplayedFirmware != "111" || got.Applicability.HardwareRevision != nil || got.Applicability.Mode != "keyboard-stick" {
		t.Fatalf("applicability: %+v", got.Applicability)
	}
	if len(got.Controls) != len(want) || len(got.Decorations) != 0 {
		t.Fatalf("controls/decorations: %d/%d", len(got.Controls), len(got.Decorations))
	}
	byID := make(map[string]Control, len(got.Controls))
	sources := make(map[int]bool, len(got.Controls))
	for _, control := range got.Controls {
		if _, exists := byID[control.ID]; exists || sources[control.SourceInputID] {
			t.Fatalf("duplicate id or source: %s/%d", control.ID, control.SourceInputID)
		}
		byID[control.ID] = control
		sources[control.SourceInputID] = true
	}
	for _, expected := range want {
		control, ok := byID[expected.id]
		if !ok {
			t.Errorf("missing control %s", expected.id)
			continue
		}
		shape := control.Shape
		if control.Group != expected.group || control.SourceInputID != expected.source || control.PinOne != expected.pin1 || control.PinTwo != expected.pin2 || control.ZIndex != 0 || shape.Type != "rounded_rect" || shape.X != expected.x || shape.Y != expected.y || shape.Width != expected.width || shape.Height != expected.height {
			t.Errorf("%s correspondence or rectangle: %+v", expected.id, control)
		}
		if control.LabelAnchor != (Point{expected.x + expected.width/2, expected.y + expected.height/2}) {
			t.Errorf("%s center anchor: %+v", expected.id, control.LabelAnchor)
		}
		if shape.X < got.ViewBox.X || shape.Y < got.ViewBox.Y || shape.X+shape.Width > got.ViewBox.X+got.ViewBox.Width || shape.Y+shape.Height > got.ViewBox.Y+got.ViewBox.Height {
			t.Errorf("%s rectangle outside view box", expected.id)
		}
	}
	for _, source := range []int{21, 25, 26, 27, 32, 33, 34, 35, 39, 40, 42, 43} {
		if sources[source] {
			t.Errorf("excluded source %d has a control", source)
		}
	}
	got.Controls[0].ID = "changed"
	got.Controls[0].SourceInputID = 999
	got.Controls[0].Shape.Width = 1
	again, err := LoadEmbedded("cyborg-ii", "left")
	if err != nil {
		t.Fatal(err)
	}
	if again.Controls[0].ID == "changed" || again.Controls[0].SourceInputID == 999 || again.Controls[0].Shape.Width == 1 {
		t.Fatal("later load was changed by caller mutation")
	}

	for _, tc := range []struct {
		name, model, hand, path string
	}{
		{"right hand", "cyborg-ii", "right", "hand"},
		{"unknown model", "unknown", "left", "model"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadEmbedded(tc.model, tc.hand)
			var layoutErr *Error
			if !errors.As(err, &layoutErr) || layoutErr.Code != ERR_LAYOUT_UNSUPPORTED || layoutErr.Path != tc.path {
				t.Fatalf("want unsupported at %s, got %v", tc.path, err)
			}
		})
	}
}
