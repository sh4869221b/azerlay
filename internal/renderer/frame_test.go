package renderer

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/profile"
)

func frameImage(t *testing.T, frame *Frame, width, height int) image.Image {
	t.Helper()
	surface := cairo.CreateImageSurface(cairo.FormatARGB32, width, height)
	defer surface.Close()
	context := cairo.Create(surface)
	frame.Draw(context)
	if status := context.Status(); status != cairo.StatusSuccess {
		t.Fatalf("Cairo frame draw: %v", status)
	}
	context.Close()
	var encoded bytes.Buffer
	if err := surface.WriteToPNGWriter(&encoded); err != nil {
		t.Fatal(err)
	}
	result, err := png.Decode(&encoded)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func imagesDifferIn(a, b image.Image, left, top, right, bottom int) bool {
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			if pixel(a, x, y) != pixel(b, x, y) {
				return true
			}
		}
	}
	return false
}

func TestStatusLayout(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	definition := testTextDefinition(160, 90)
	options := Options{Scale: 1, Opacity: 1, FontScale: 1, Mode: "detailed", Theme: "dark", ShowProfileName: true, ShowStatus: true}
	for _, status := range []Status{StatusNone, StatusDisconnected, StatusProfileMissing, StatusReloadFailed} {
		content := Content{ProfileName: "Synthetic profile", Status: status, Controls: map[string]Control{"button": {Known: true, Down: true, Assignments: []Assignment{{Trigger: profile.TriggerLong, Kind: profile.BindingMacro, Label: "日本語 action", BindingDisplay: "Macro"}}}}}
		frame := Prepare(NewSnapshot(definition, content), options, 392, 348)
		if frame == nil || frame.title == nil || len(frame.controls[0].lines) < 2 || frame.controls[0].lines[0].text != "日本語 action" {
			t.Fatalf("status %s lost prepared title or control content", status)
		}
		if frame.title.y+frame.title.height > definition.ViewBox.Y || frame.title.layout.UnknownGlyphsCount() != 0 {
			t.Fatalf("title overlaps diagram or lacks glyph: %+v", frame.title)
		}
		if status != StatusNone {
			if frame.status == nil || frame.statusBand <= 0 || frame.status.y < definition.ViewBox.Y+definition.ViewBox.Height || frame.status.layout.UnknownGlyphsCount() != 0 {
				t.Fatalf("status %s missing measured band: %+v", status, frame.status)
			}
		} else if frame.status != nil {
			t.Fatal("none status drew a band")
		}
		image := frameImage(t, frame, 392, 348)
		if pixel(image, 196, 174).A == 0 {
			t.Fatalf("status %s produced an empty Cairo image", status)
		}
		if status == StatusReloadFailed {
			if dir := os.Getenv("AZERLAY_RENDER_QA_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				file, err := os.Create(filepath.Join(dir, "detailed-status.png"))
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(file, image); err != nil {
					file.Close()
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestTitleDescenderNotClipped(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	definition := layout.Definition{ViewBox: layout.Rect{Width: 200, Height: 80}}
	frame := Prepare(NewSnapshot(definition, Content{ProfileName: "gjpqy"}), Options{Scale: 1, Opacity: 1, FontScale: 2, Theme: "dark", ShowProfileName: true}, 216, 200)
	if frame == nil || frame.title == nil {
		t.Fatal("descender title was not prepared")
	}
	image := frameImage(t, frame, 216, 200)
	oldClipBottom := int(math.Ceil(frame.placement.Y + (definition.ViewBox.Y-geometryPadding)*frame.placement.Scale))
	newClipBottom := int(math.Ceil(frame.placement.Y + definition.ViewBox.Y*frame.placement.Scale))
	for y := oldClipBottom; y < newClipBottom; y++ {
		for x := 0; x < 216; x++ {
			color := pixel(image, x, y)
			if color.R > 0x80 && color.G > 0x80 && color.B > 0x80 {
				return
			}
		}
	}
	t.Fatal("title descenders were cropped above the diagram")
}

func TestTextOverflow(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	narrow := testTextDefinition(20, 22)
	content := Content{Controls: map[string]Control{"button": {Known: true, Down: true, Assignments: []Assignment{{Kind: profile.BindingKeyboard, Label: strings.Repeat("Very long synthetic action ", 5), BindingDisplay: "W"}}}}}
	frame := Prepare(NewSnapshot(narrow, content), Options{Scale: 1, Opacity: 1, FontScale: 4, Mode: "detailed"}, 56, 58)
	if frame == nil || len(frame.controls[0].lines) != 0 || frame.controls[0].style.physical != physicalPressed {
		t.Fatalf("small control displaced its physical mark with text: %+v", frame.controls[0])
	}
	minuscule := Prepare(NewSnapshot(testTextDefinition(70, 100), content), Options{Scale: 1, Opacity: 1, FontScale: 1, Mode: "normal"}, 1, 1)
	if minuscule == nil || len(minuscule.controls[0].lines) != 0 {
		t.Fatal("text with less than one device pixel of height was retained")
	}
	wide := testTextDefinition(70, 100)
	smallFont := Prepare(NewSnapshot(wide, content), Options{Scale: 1, Opacity: 1, FontScale: .25, Mode: "normal"}, 106, 136)
	largeFont := Prepare(NewSnapshot(wide, content), Options{Scale: 1, Opacity: 1, FontScale: 4, Mode: "normal"}, 106, 136)
	if len(smallFont.controls[0].lines) != 2 || len(largeFont.controls[0].lines) != 1 || largeFont.controls[0].lines[0].height <= smallFont.controls[0].lines[0].height {
		t.Fatalf("font scale changed line priority or was silently shrunk: small=%+v large=%+v", smallFont.controls[0].lines, largeFont.controls[0].lines)
	}
	if !smallFont.controls[0].lines[0].ellipsized || !largeFont.controls[0].lines[0].ellipsized {
		t.Fatal("long label failed to ellipsize")
	}
	polygon := layout.Definition{ViewBox: layout.Rect{Width: 120, Height: 120}, Controls: []layout.Control{{ID: "button", Shape: layout.Shape{Type: "polygon", Points: []layout.Point{{X: 10, Y: 10}, {X: 110, Y: 10}, {X: 60, Y: 110}}}, LabelAnchor: layout.Point{X: 60, Y: 70}}}}
	polygonContent := Content{Controls: map[string]Control{"button": {Known: true, Assignments: []Assignment{{Kind: profile.BindingKeyboard, Label: strings.Repeat("WIDE ", 12)}}}}}
	withText := Prepare(NewSnapshot(polygon, polygonContent), Options{Scale: 1, Opacity: 1, FontScale: 1, Mode: "compact"}, 136, 136)
	withoutText := Prepare(NewSnapshot(polygon, polygonContent), Options{Scale: 1, Opacity: 1, FontScale: 1, Mode: "compact"}, 136, 136)
	if len(withText.controls[0].lines) != 1 {
		t.Fatal("polygon text was not prepared")
	}
	withoutText.controls[0].lines = nil
	painted := frameImage(t, withText, 136, 136)
	bare := frameImage(t, withoutText, 136, 136)
	if imagesDifferIn(painted, bare, 24, 65, 43, 90) || !imagesDifferIn(painted, bare, 52, 65, 84, 90) {
		t.Fatal("polygon text escaped shape clip or did not render inside it")
	}
}

func TestVisibilityFlags(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	definition := layout.Definition{ViewBox: layout.Rect{Width: 160, Height: 120}, Controls: []layout.Control{
		{ID: "unbound", Shape: layout.Shape{Type: "rounded_rect", X: 5, Y: 10, Width: 70, Height: 100}, LabelAnchor: layout.Point{X: 40, Y: 60}},
		{ID: "unknown", Shape: layout.Shape{Type: "rounded_rect", X: 85, Y: 10, Width: 70, Height: 100}, LabelAnchor: layout.Point{X: 120, Y: 60}},
	}}
	content := Content{ProfileName: "Synthetic", Status: StatusDisconnected, Controls: map[string]Control{
		"unbound": {Known: true, Down: true, Assignments: []Assignment{{Kind: profile.BindingUnbound, Label: "Unbound", Ambiguous: true}}},
		"unknown": {Assignments: []Assignment{{Kind: profile.BindingUnknown, Label: "Unknown"}}},
	}}
	hidden := Prepare(NewSnapshot(definition, content), Options{Scale: 1, Opacity: 1, FontScale: 1, Mode: "detailed"}, 176, 136)
	if hidden.title != nil || hidden.status != nil || len(hidden.controls[0].lines) != 0 || hidden.controls[0].style.static != "" || hidden.controls[0].style.ambiguous || hidden.controls[0].style.physical != physicalPressed || len(hidden.controls[1].lines) == 0 {
		t.Fatalf("visibility flags hid physical observation or unknown binding: %+v", hidden)
	}
	shown := Prepare(NewSnapshot(definition, content), Options{Scale: 1, Opacity: 1, FontScale: 1, Mode: "detailed", ShowUnbound: true, ShowAmbiguous: true, ShowProfileName: true, ShowStatus: true}, 176, 200)
	if shown.title == nil || shown.status == nil || len(shown.controls[0].lines) == 0 || shown.controls[0].style.static != profile.BindingUnbound || !shown.controls[0].style.ambiguous || shown.controls[0].style.physical != hidden.controls[0].style.physical {
		t.Fatalf("visible static metadata or physical observation changed: %+v", shown)
	}
}
