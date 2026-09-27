package renderer

import (
	"bytes"
	"image"
	stdcolor "image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/profile"
)

func renderedGeometry(t *testing.T, snapshot *OverlaySnapshot, options Options, width, height int) image.Image {
	t.Helper()
	surface := cairo.CreateImageSurface(cairo.FormatARGB32, width, height)
	defer surface.Close()
	context := cairo.Create(surface)
	drawGeometry(context, snapshot, options, float64(width), float64(height), 0, 0)
	if err := context.Status(); err != cairo.StatusSuccess {
		t.Fatalf("Cairo drawing failed: %v", err)
	}
	context.Close()
	surface.Flush()
	var encoded bytes.Buffer
	if err := surface.WriteToPNGWriter(&encoded); err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(&encoded)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func pixel(image image.Image, x, y int) stdcolor.NRGBA {
	return stdcolor.NRGBAModel.Convert(image.At(x, y)).(stdcolor.NRGBA)
}

func TestPrimitiveRaster(t *testing.T) {
	t.Parallel()
	definition := syntheticShapes(t)
	primitiveContent := Content{Controls: make(map[string]Control, len(definition.Controls))}
	for _, region := range definition.Controls {
		primitiveContent.Controls[region.ID] = Control{Known: true, Down: region.ID == "circle" || region.ID == "group" || region.ID == "path", Assignments: []Assignment{{Kind: profile.BindingKeyboard}}}
	}
	snapshot := NewSnapshot(definition, primitiveContent)
	image := renderedGeometry(t, snapshot, Options{Scale: 1, Opacity: 1, Theme: "dark", ShowUnbound: true}, 116, 116)
	if dir := os.Getenv("AZERLAY_RENDER_QA_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		file, err := os.Create(filepath.Join(dir, "primitive-raster.png"))
		if err != nil {
			t.Fatal(err)
		}
		preview := renderedGeometry(t, snapshot, Options{Scale: 1, Opacity: 1, Theme: "dark"}, 464, 464)
		if err := png.Encode(file, preview); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	background := pixel(image, 110, 50)
	for name, point := range map[string][2]int{
		"rounded rect": {18, 20}, "polygon": {43, 11}, "circle": {68, 18},
		"ellipse": {93, 18}, "line": {18, 43}, "path": {43, 43},
		"transformed group": {83, 83}, "decoration": {58, 108},
	} {
		if got := pixel(image, point[0], point[1]); got == background || got.A == 0 {
			t.Errorf("%s did not paint at %v: pixel %+v", name, point, got)
		}
	}

	front := layout.Control{ID: "front", ZIndex: 2, Shape: layout.Shape{Type: "rounded_rect", X: 40, Y: 40, Width: 20, Height: 20}}
	back := layout.Control{ID: "back", ZIndex: 1, Shape: front.Shape}
	definition = layout.Definition{ViewBox: layout.Rect{Width: 100, Height: 100}, Controls: []layout.Control{front, back}}
	content := Content{Controls: map[string]Control{
		"front": {Known: true, Down: true, Assignments: []Assignment{{Kind: profile.BindingKeyboard}}},
		"back":  {Known: true, Assignments: []Assignment{{Kind: profile.BindingKeyboard}}},
	}}
	first := renderedGeometry(t, NewSnapshot(definition, content), Options{Scale: 1, Opacity: 1, Theme: "dark"}, 116, 116)
	definition.Controls[0], definition.Controls[1] = definition.Controls[1], definition.Controls[0]
	second := renderedGeometry(t, NewSnapshot(definition, content), Options{Scale: 1, Opacity: 1, Theme: "dark"}, 116, 116)
	if pixel(first, 58, 58) != pixel(second, 58, 58) || pixel(first, 58, 58) != (stdcolor.NRGBA{R: 0x1d, G: 0x4e, B: 0xd8, A: 0xff}) {
		t.Fatalf("z order changed with source order: %v, %v", pixel(first, 58, 58), pixel(second, 58, 58))
	}
}

func TestThemeStates(t *testing.T) {
	t.Parallel()
	definition := layout.Definition{ViewBox: layout.Rect{Width: 100, Height: 100}, Controls: []layout.Control{{ID: "button", Shape: layout.Shape{Type: "rounded_rect", X: 10, Y: 10, Width: 30, Height: 30}}}}
	snapshot := NewSnapshot(definition, Content{Controls: map[string]Control{"button": {Known: true, Down: true, Assignments: []Assignment{{Kind: profile.BindingKeyboard}}}}})
	options := Options{Scale: 1, Opacity: 1, Theme: "dark"}
	dark := renderedGeometry(t, snapshot, options, 116, 116)
	options.Theme = "light"
	light := renderedGeometry(t, snapshot, options, 116, 116)
	options.HighContrast = true
	highContrast := renderedGeometry(t, snapshot, options, 116, 116)
	if pixel(dark, 100, 100) != (stdcolor.NRGBA{R: 0x11, G: 0x18, B: 0x27, A: 0xff}) || pixel(light, 100, 100) != (stdcolor.NRGBA{R: 0xf8, G: 0xfa, B: 0xfc, A: 0xff}) || pixel(highContrast, 100, 100) != (stdcolor.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}) {
		t.Fatal("background theme did not reach Cairo surface")
	}
	if pixel(dark, 42, 42) != (stdcolor.NRGBA{R: 0x1d, G: 0x4e, B: 0xd8, A: 0xff}) || pixel(light, 42, 42) != (stdcolor.NRGBA{R: 0xbf, G: 0xdb, B: 0xfe, A: 0xff}) || pixel(highContrast, 42, 42) != (stdcolor.NRGBA{R: 0xba, G: 0xe6, B: 0xfd, A: 0xff}) {
		t.Fatal("pressed theme did not reach Cairo surface")
	}
}

func TestZeroAllocation(t *testing.T) {
	t.Parallel()
	surface := cairo.CreateImageSurface(cairo.FormatARGB32, 8, 8)
	defer surface.Close()
	context := cairo.Create(surface)
	defer context.Close()
	context.SetSourceRGB(1, 0, 0)
	context.Paint()
	snapshot := NewSnapshot(syntheticShapes(t), Content{})
	for _, size := range [][2]float64{{0, 8}, {8, 0}, {-1, 8}, {8, -1}} {
		drawGeometry(context, snapshot, Options{Scale: 1, Opacity: 1}, size[0], size[1], 0, 0)
	}
	surface.Flush()
	data := surface.Data()
	if data[0] != 0 || data[1] != 0 || data[2] != 0xff || data[3] != 0xff {
		t.Fatalf("zero allocation changed destination pixel: %v", data[:4])
	}
}

func TestPhysicalStyleIndependence(t *testing.T) {
	t.Parallel()
	definition := layout.Definition{ViewBox: layout.Rect{Width: 100, Height: 100}, Controls: []layout.Control{{ID: "button", Shape: layout.Shape{Type: "rounded_rect", X: 10, Y: 10, Width: 30, Height: 30}}}}
	options := Options{Scale: 1, Opacity: 1, Theme: "dark", ShowUnbound: true, ShowAmbiguous: true}
	render := func(control Control) image.Image {
		return renderedGeometry(t, NewSnapshot(definition, Content{Controls: map[string]Control{"button": control}}), options, 116, 116)
	}
	keyboard := render(Control{Known: true, Down: true, Assignments: []Assignment{{Kind: profile.BindingKeyboard}}})
	unbound := render(Control{Known: true, Down: true, Assignments: []Assignment{{Kind: profile.BindingUnbound}}})
	if pixel(keyboard, 24, 24) != pixel(unbound, 24, 24) || pixel(keyboard, 35, 35) != pixel(unbound, 35, 35) {
		t.Fatal("unbound assignment changed observed physical pressed mark or fill")
	}
	released := render(Control{Known: true, Assignments: []Assignment{{Kind: profile.BindingKeyboard}}})
	ambiguous := render(Control{Known: true, Assignments: []Assignment{{Kind: profile.BindingKeyboard, Ambiguous: true}}})
	if pixel(released, 24, 24) != pixel(ambiguous, 24, 24) || pixel(released, 35, 35) != pixel(ambiguous, 35, 35) {
		t.Fatal("static ambiguity changed observed release mark or fill")
	}
	unknown := render(Control{Down: true, Assignments: []Assignment{{Kind: profile.BindingKeyboard}}})
	if pixel(unknown, 24, 24) == pixel(keyboard, 24, 24) || pixel(unknown, 24, 24) == pixel(released, 24, 24) {
		t.Fatal("unknown Down rendered as observed pressed or released")
	}
}

func TestOpacityOnce(t *testing.T) {
	t.Parallel()
	snapshot := NewSnapshot(syntheticShapes(t), Content{})
	for _, test := range []struct {
		opacity float64
		alpha   uint8
	}{{0, 0}, {.5, 128}, {1, 255}} {
		image := renderedGeometry(t, snapshot, Options{Scale: 1, Opacity: test.opacity, Theme: "dark"}, 116, 116)
		if got := pixel(image, 110, 50).A; got != test.alpha {
			t.Fatalf("opacity %g yielded alpha %d, want %d", test.opacity, got, test.alpha)
		}
	}
}
