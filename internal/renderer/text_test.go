package renderer

import (
	"encoding/json"
	"os"
	"reflect"
	"runtime"
	"testing"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/profile"
)

func testTextDefinition(width, height float64) layout.Definition {
	return layout.Definition{
		ViewBox:  layout.Rect{Width: width + 20, Height: height + 20},
		Controls: []layout.Control{{ID: "button", Shape: layout.Shape{Type: "rounded_rect", X: 10, Y: 10, Width: width, Height: height, Radius: 4}, LabelAnchor: layout.Point{X: 10 + width/2, Y: 10 + height/2}}},
	}
}

func textValues(specs []textSpec) []string {
	values := make([]string, len(specs))
	for i, spec := range specs {
		values[i] = spec.text
	}
	return values
}

func TestDensityTextGolden(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/text.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		Compact      []string `json:"compact"`
		Normal       []string `json:"normal"`
		Detailed     []string `json:"detailed"`
		MacroPrimary []string `json:"macro_primary"`
	}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	control := Control{Assignments: []Assignment{
		{Trigger: profile.TriggerDouble, Kind: profile.BindingMacro, Label: "Double action", BindingDisplay: "Macro"},
		{Trigger: profile.TriggerUnknown, Kind: profile.BindingUnknown, Label: "Unknown action", BindingDisplay: "?"},
		{Trigger: profile.TriggerLong, Kind: profile.BindingKeyboard, Label: "Hold action", BindingDisplay: "U"},
		{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Label: "Single action", BindingDisplay: "W", Ambiguous: true},
	}}
	for _, test := range []struct {
		mode string
		want []string
	}{{"compact", want.Compact}, {"normal", want.Normal}, {"detailed", want.Detailed}} {
		if got := textValues(controlText(control, Options{Mode: test.mode, ShowAmbiguous: true})); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s selection: got %v, want %v", test.mode, got, test.want)
		}
	}
	macro := Control{Assignments: []Assignment{{Trigger: profile.TriggerLong, Kind: profile.BindingMacro, Label: "Macro action", BindingDisplay: "Macro", Ambiguous: true}}}
	if got := textValues(controlText(macro, Options{Mode: "detailed", ShowAmbiguous: true})); !reflect.DeepEqual(got, want.MacroPrimary) {
		t.Fatalf("static macro metadata: got %v, want %v", got, want.MacroPrimary)
	}
}

func TestUnicodeLayout(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	definition := testTextDefinition(260, 90)
	label := "日本語 e\u0301 👾 <b>literal</b>\nnext\tpart " + "長いラベル長いラベル長いラベル"
	snapshot := NewSnapshot(definition, Content{Controls: map[string]Control{"button": {Known: true, Assignments: []Assignment{{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Label: label, BindingDisplay: "W"}}}}})
	frame := Prepare(snapshot, Options{Scale: 1, Opacity: 1, FontScale: 1, Mode: "normal"}, 296, 126)
	if frame == nil || len(frame.controls) != 1 || len(frame.controls[0].lines) != 2 {
		t.Fatalf("Unicode frame missing lines: %+v", frame)
	}
	line := frame.controls[0].lines[0]
	if line.text != "日本語 e\u0301 👾 <b>literal</b> next part 長いラベル長いラベル長いラベル" || line.layout.Text() != line.text || !line.ellipsized {
		t.Fatalf("plain Unicode/ellipsis lost: text=%q ellipsized=%t", line.layout.Text(), line.ellipsized)
	}
	_, logical := line.layout.PixelExtents()
	if float64(logical.Width()) > line.width+1 || line.layout.UnknownGlyphsCount() != 0 {
		t.Fatalf("Unicode bounds/glyphs: width=%d allowed=%g unknown=%d", logical.Width(), line.width, line.layout.UnknownGlyphsCount())
	}
	surface := cairo.CreateImageSurface(cairo.FormatARGB32, 1, 1)
	defer surface.Close()
	context := cairo.Create(surface)
	defer context.Close()
	japanese := prepareLine(context, textSpec{text: "日本語", size: 14}, Options{FontScale: 1}, 100, rgb(0xffffff))
	if japanese.layout.UnknownGlyphsCount() != 0 {
		t.Fatal("Japanese font fallback is missing")
	}
}

func TestEmptyLabelDisplaysAssignment(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	control := Control{Assignments: []Assignment{
		{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, BindingDisplay: "W"},
		{Trigger: profile.TriggerLong, Kind: profile.BindingKeyboard, BindingDisplay: "Left Ctrl+U"},
	}}
	definition := testTextDefinition(260, 90)
	for _, test := range []struct {
		mode string
		want []string
	}{
		{"compact", []string{"W"}},
		{"normal", []string{"W"}},
		{"detailed", []string{"W", "HOLD: Left Ctrl+U"}},
	} {
		t.Run(test.mode, func(t *testing.T) {
			options := Options{Scale: 1, Opacity: 1, FontScale: 1, Mode: test.mode}
			if got := textValues(controlText(control, options)); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("empty label: got %v, want %v", got, test.want)
			}
			frame := Prepare(NewSnapshot(definition, Content{Controls: map[string]Control{"button": control}}), options, 296, 126)
			if frame == nil || len(frame.controls) != 1 || len(frame.controls[0].lines) != len(test.want) {
				t.Fatalf("assignment frame missing lines: %+v", frame)
			}
			for i, want := range test.want {
				if got := frame.controls[0].lines[i].layout.Text(); got != want {
					t.Fatalf("rendered line %d: got %q, want %q", i, got, want)
				}
			}
			if control.Assignments[0].Label != "" || control.Assignments[1].Label != "" {
				t.Fatal("rendering populated an empty label")
			}
		})
	}
}

func TestMultiTriggerPriority(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	control := Control{Known: true, Assignments: []Assignment{
		{Trigger: profile.TriggerDouble, Kind: profile.BindingKeyboard, Label: "Double", BindingDisplay: "D"},
		{Trigger: profile.TriggerLong, Kind: profile.BindingKeyboard, Label: "Hold first", BindingDisplay: "H"},
		{Trigger: profile.TriggerLong, Kind: profile.BindingKeyboard, Label: "Hold second", BindingDisplay: "J"},
		{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Label: "Primary", BindingDisplay: "P"},
	}}
	want := []string{"Primary", "P", "HOLD: Hold first · H", "HOLD: Hold second · J", "DOUBLE: Double · D"}
	if got := textValues(controlText(control, Options{Mode: "detailed"})); !reflect.DeepEqual(got, want) {
		t.Fatalf("trigger priority/source order: got %v, want %v", got, want)
	}
	definition := testTextDefinition(70, 50)
	frame := Prepare(NewSnapshot(definition, Content{Controls: map[string]Control{"button": control}}), Options{Scale: 1, Opacity: 1, FontScale: 1, Mode: "detailed"}, 106, 86)
	if frame == nil || len(frame.controls[0].lines) != 1 || frame.controls[0].lines[0].text != "Primary" {
		t.Fatalf("short control did not retain only primary action: %+v", frame.controls[0].lines)
	}
}
