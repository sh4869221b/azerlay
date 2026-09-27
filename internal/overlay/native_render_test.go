package overlay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/renderer"
)

func TestNativeRenderer(t *testing.T) {
	if os.Getenv("AZERLAY_TEST_WAYLAND_DISPLAY") == "" {
		launcher, err := filepath.Abs("../../scripts/test-wayland.sh")
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(t.Context(), "bash", launcher, os.Args[0], "-test.run=^TestNativeRenderer$", "-test.v")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Wayland fixture: %v\n%s", err, output)
		} else {
			t.Log(string(output))
		}
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0])
	cmd.Env = append(os.Environ(), nativeChildEnv+"=renderer", "GDK_BACKEND=wayland", "WAYLAND_DISPLAY="+os.Getenv("AZERLAY_TEST_WAYLAND_DISPLAY"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native renderer: %v\n%s", err, output)
	}
	if string(output) != "rendered\n" {
		t.Fatalf("native renderer did not complete: %s", output)
	}
}

func renderPixel(frame *renderer.Frame, width, height, x, y int) ([4]uint8, error) {
	surface := cairo.CreateImageSurface(cairo.FormatARGB32, width, height)
	defer surface.Close()
	cr := cairo.Create(surface)
	frame.Draw(cr)
	cr.Close()
	var encoded bytes.Buffer
	if err := surface.WriteToPNGWriter(&encoded); err != nil {
		return [4]uint8{}, err
	}
	image, err := png.Decode(&encoded)
	if err != nil {
		return [4]uint8{}, err
	}
	r, g, b, a := image.At(x, y).RGBA()
	return [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}, nil
}

func runNativeRendererChild() int {
	settings := config.Overlay{Anchor: "center", Scale: 1, Opacity: .75, Mode: "detailed", ShowProfileName: true, ShowStatus: true}
	w, err := New(settings)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer w.Close()
	definition := layout.Definition{ViewBox: layout.Rect{Width: 160, Height: 100}, Controls: []layout.Control{{ID: "button", Shape: layout.Shape{Type: "rounded_rect", X: 15, Y: 15, Width: 130, Height: 70}, LabelAnchor: layout.Point{X: 80, Y: 50}}}}
	snapshot := renderer.NewSnapshot(definition, renderer.Content{ProfileName: "Synthetic", Status: renderer.StatusDisconnected, Controls: map[string]renderer.Control{"button": {Known: true, Down: true, Assignments: []renderer.Assignment{{Kind: profile.BindingKeyboard, Label: "日本語 action", BindingDisplay: "K"}}}}})
	appearance := config.Appearance{Theme: "dark", FontScale: 1}
	w.SetRenderState(snapshot, appearance, true)
	if w.area == nil || w.frame != nil || w.area.ContentHeight() <= 150 {
		fmt.Fprintln(os.Stderr, "render area was not attached with deferred frame")
		return 1
	}
	if err := w.SetVisible(true); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	loop := glib.NewMainLoop(nil, false)
	deadline := time.Now().Add(10 * time.Second)
	phase := 0
	var priorFrame *renderer.Frame
	var priorWidget any
	var dark [4]uint8
	var failure error
	check := func(ok bool, message string) bool {
		if ok {
			return true
		}
		failure = errors.New(message)
		loop.Quit()
		return false
	}
	glib.TimeoutAdd(10, func() bool {
		if time.Now().After(deadline) {
			failure = errors.New("native renderer timed out")
			loop.Quit()
			return false
		}
		switch phase {
		case 0:
			if !mappedLayer(w) || w.frame == nil {
				break
			}
			if !check(w.area != nil && w.area.Mapped() && w.renderWidth > 0 && w.renderHeight > 0, "mapped renderer lacks allocated child") {
				return false
			}
			dark, err = renderPixel(w.frame, w.renderWidth, w.renderHeight, w.renderWidth/2, w.renderHeight/2)
			if !check(err == nil && dark[3] > 0 && dark[3] < 255, "renderer did not draw with configured opacity") {
				return false
			}
			priorFrame = w.frame
			w.SetRenderState(snapshot, appearance, true)
			if !check(w.frame == priorFrame, "unchanged render state was prepared twice") {
				return false
			}
			w.area.SetContentWidth(w.renderWidth + 64)
			phase = 1
		case 1:
			if w.frame == priorFrame || w.renderWidth <= 176 {
				break
			}
			priorFrame = w.frame
			appearance.Theme = "light"
			appearance.FontScale = 1.5
			w.SetRenderState(snapshot, appearance, false)
			if !check(w.frame != priorFrame, "appearance update reused stale frame") {
				return false
			}
			light, drawErr := renderPixel(w.frame, w.renderWidth, w.renderHeight, w.renderWidth/2, w.renderHeight/2)
			if !check(drawErr == nil && light != dark, "appearance update did not change native raster") {
				return false
			}
			priorFrame = w.frame
			settings.Mode = "compact"
			settings.Scale = 2
			oldContentWidth := w.area.ContentWidth()
			if !check(w.ApplyConfig(settings) == nil && w.frame != priorFrame && w.area.ContentWidth() > oldContentWidth, "overlay drawing options and requested size were not reapplied") {
				return false
			}
			if !check(w.SetVisible(false) == nil, "hide failed") {
				return false
			}
			phase = 2
		case 2:
			if w.State().Mapped {
				break
			}
			if !check(w.SetVisible(true) == nil, "show failed") {
				return false
			}
			phase = 3
		case 3:
			if !mappedLayer(w) {
				break
			}
			if !check(w.snapshot == snapshot && w.frame != nil, "hide/show dropped render state") {
				return false
			}
			priorWidget = w.widget
			settings.Monitor = "azerlay-missing-monitor"
			if !check(w.ApplyConfig(settings) == nil && w.widget == nil && w.area == nil && w.frame == nil && w.snapshot == snapshot, "missing monitor kept widget or lost snapshot") {
				return false
			}
			settings.Monitor = ""
			if !check(w.ApplyConfig(settings) == nil && w.widget != nil && w.widget != priorWidget && w.area != nil, "monitor recovery did not recreate renderer") {
				return false
			}
			phase = 4
		case 4:
			if !mappedLayer(w) || w.frame == nil {
				break
			}
			if !check(w.snapshot == snapshot && w.area != nil, "recreated widget lost render state") {
				return false
			}
			recreated, drawErr := renderPixel(w.frame, w.renderWidth, w.renderHeight, w.renderWidth/2, w.renderHeight/2)
			if !check(drawErr == nil && recreated[3] > 0, "recreated child did not render snapshot") {
				return false
			}
			w.SetRenderState(nil, appearance, false)
			if !check(w.area == nil && w.frame == nil, "nil snapshot retained renderer") {
				return false
			}
			w.SetRenderState(snapshot, appearance, false)
			if !check(w.area != nil, "reattach failed") {
				return false
			}
			w.renderWidth, w.renderHeight = 0, 0
			w.prepareRender(false)
			if !check(w.frame == nil, "zero allocation retained frame") {
				return false
			}
			w.renderWidth, w.renderHeight = 176, 116
			w.prepareRender(false)
			if !check(w.frame != nil, "positive allocation did not prepare frame") {
				return false
			}
			w.Close()
			if !check(w.area == nil && w.frame == nil && w.widget == nil, "Close retained renderer") {
				return false
			}
			fmt.Println("rendered")
			loop.Quit()
			return false
		}
		return true
	})
	loop.Run()
	if failure != nil {
		fmt.Fprintln(os.Stderr, failure)
		return 1
	}
	return 0
}
