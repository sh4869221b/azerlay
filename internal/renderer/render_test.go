package renderer

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/profile"
)

func sampleSnapshot(t *testing.T, status Status) *OverlaySnapshot {
	t.Helper()
	definition, err := layout.LoadEmbedded("cyborg-ii", "left")
	if err != nil {
		t.Fatal(err)
	}
	if len(definition.Controls) != 31 {
		t.Fatalf("embedded layout has %d regions, want 30 physical regions and stick.main", len(definition.Controls))
	}
	controls := make(map[string]Control, len(definition.Controls))
	for i, region := range definition.Controls {
		control := Control{Known: i%3 != 0, Down: i%3 == 2}
		switch i % 7 {
		case 0:
			control.Assignments = []Assignment{{Trigger: profile.TriggerSingle, Kind: profile.BindingUnbound, Label: "Unbound"}}
		case 1:
			control.Assignments = []Assignment{{Trigger: profile.TriggerUnknown, Kind: profile.BindingUnknown, Label: "Unknown"}}
		case 2:
			control.Assignments = []Assignment{{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Label: "日本語 action", BindingDisplay: "W"}}
		case 3:
			control.Assignments = []Assignment{
				{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Label: "Primary action", BindingDisplay: "Q"},
				{Trigger: profile.TriggerLong, Kind: profile.BindingMacro, Label: "Hold macro", BindingDisplay: "Macro"},
				{Trigger: profile.TriggerDouble, Kind: profile.BindingKeyboard, Label: "Double action", BindingDisplay: "D"},
			}
		case 4:
			control.Assignments = []Assignment{{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Label: "Duplicate", BindingDisplay: "E", Ambiguous: true}}
		case 5:
			control.Assignments = []Assignment{{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Label: "長い日本語と English label é 👾 that must ellipsize", BindingDisplay: "R"}}
		default:
			control.Assignments = []Assignment{{Trigger: profile.TriggerSingle, Kind: profile.BindingMacro, Label: "Macro", BindingDisplay: "Macro"}}
		}
		controls[region.ID] = control
	}
	controls["stick.main"] = Control{Assignments: []Assignment{{Kind: profile.BindingStick, Label: "Stick", BindingDisplay: "Analog"}}}
	return NewSnapshot(definition, Content{ProfileName: "Synthetic Cyborg II · 日本語", Controls: controls, Status: status})
}

func TestRenderSamples(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	dir := os.Getenv("AZERLAY_RENDER_QA_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	base := Options{Scale: 1, Opacity: .88, FontScale: 1, ShowProfileName: true, ShowStatus: true, ShowUnbound: true, ShowAmbiguous: true}
	for _, sample := range []struct {
		name          string
		status        Status
		mode, theme   string
		highContrast  bool
		fontScale     float64
		width, height int
	}{
		{"normal-dark", StatusNone, "normal", "dark", false, 1, 710, 710},
		{"compact-light", StatusNone, "compact", "light", false, 1, 710, 710},
		{"detailed-dark-hc", StatusNone, "detailed", "dark", true, 1, 710, 710},
		{"detailed-light-hc", StatusNone, "detailed", "light", true, 1, 710, 710},
		{"status-disconnected", StatusDisconnected, "normal", "dark", false, 1, 710, 710},
		{"status-profile-missing", StatusProfileMissing, "normal", "dark", false, 1, 710, 710},
		{"status-reload-failed", StatusReloadFailed, "detailed", "dark", false, 1, 710, 710},
		{"font-large-narrow", StatusReloadFailed, "detailed", "dark", false, 4, 360, 400},
	} {
		t.Run(sample.name, func(t *testing.T) {
			options := base
			options.Mode, options.Theme = sample.mode, sample.theme
			options.HighContrast, options.FontScale = sample.highContrast, sample.fontScale
			frame := Prepare(sampleSnapshot(t, sample.status), options, float64(sample.width), float64(sample.height))
			if frame == nil || len(frame.controls) != 31 {
				t.Fatal("embedded scene lost regions")
			}
			stickStatic := false
			for _, control := range frame.controls {
				if control.region.ID == "stick.main" && control.style.physical == physicalNone {
					stickStatic = true
				}
			}
			if !stickStatic {
				t.Fatal("stick.main gained a physical marker")
			}
			surface := cairo.CreateImageSurface(cairo.FormatARGB32, sample.width, sample.height)
			defer surface.Close()
			cr := cairo.Create(surface)
			frame.Draw(cr)
			if status := cr.Status(); status != cairo.StatusSuccess {
				t.Fatalf("Cairo draw: %v", status)
			}
			cr.Close()
			path := filepath.Join(dir, sample.name+".png")
			if err := surface.WriteToPNG(path); err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			image, err := png.Decode(file)
			file.Close()
			if err != nil {
				t.Fatal(err)
			}
			_, _, _, alpha := image.At(sample.width/2, sample.height/2).RGBA()
			if alpha == 0 {
				t.Fatal("sample produced no Cairo pixels")
			}
		})
	}
}

func TestOverlappingControlTextZOrder(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	definition := layout.Definition{ViewBox: layout.Rect{Width: 100, Height: 100}, Controls: []layout.Control{
		{ID: "back", ZIndex: 0, Shape: layout.Shape{Type: "rounded_rect", X: 8, Y: 8, Width: 84, Height: 84}, LabelAnchor: layout.Point{X: 50, Y: 50}},
		{ID: "front", ZIndex: 1, Shape: layout.Shape{Type: "rounded_rect", X: 55, Y: 30, Width: 37, Height: 62}, LabelAnchor: layout.Point{X: 73, Y: 61}},
	}}
	render := func(definition layout.Definition, label string) image.Image {
		return renderedGeometry(t, NewSnapshot(definition, Content{Controls: map[string]Control{
			"back":  {Known: true, Assignments: []Assignment{{Kind: profile.BindingKeyboard, Label: label}}},
			"front": {Known: true, Assignments: []Assignment{{Kind: profile.BindingKeyboard, Label: "Front"}}},
		}}), Options{Scale: 1, Opacity: 1, FontScale: 1, Mode: "compact", Theme: "dark"}, 116, 116)
	}
	backOnly := definition
	backOnly.Controls = definition.Controls[:1]
	bareA, bareB := render(backOnly, "MMMMMMMM"), render(backOnly, "iiiiiiii")
	if !imagesDifferIn(bareA, bareB, 64, 42, 94, 88) {
		t.Fatal("back control labels did not cover the overlap before front control was added")
	}
	a, b := render(definition, "MMMMMMMM"), render(definition, "iiiiiiii")
	if imagesDifferIn(a, b, 64, 42, 94, 88) {
		t.Fatal("lower Z control text painted over front control")
	}
}
