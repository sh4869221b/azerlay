package layershell

import (
	"testing"

	gdk "github.com/diamondburned/gotk4/pkg/gdk/v4"
	gtk "github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func TestPlacement(t *testing.T) {
	t.Parallel()
	base := PlacementInput{MarginX: 24, MarginY: -24, MonitorWidth: 2560, MonitorHeight: 1440, WindowWidth: 160, WindowHeight: 80, HasGeometry: true}
	tests := []struct {
		anchor string
		want   Placement
	}{
		{"top-left", Placement{Left: true, Top: true, LeftMargin: 24, TopMargin: -24}},
		{"top", Placement{Left: true, Top: true, LeftMargin: 1224, TopMargin: -24}},
		{"top-right", Placement{Right: true, Top: true, RightMargin: 24, TopMargin: -24}},
		{"left", Placement{Left: true, Top: true, LeftMargin: 24, TopMargin: 656}},
		{"center", Placement{Left: true, Top: true, LeftMargin: 1224, TopMargin: 656}},
		{"right", Placement{Right: true, Top: true, RightMargin: 24, TopMargin: 656}},
		{"bottom-left", Placement{Left: true, Bottom: true, LeftMargin: 24, BottomMargin: -24}},
		{"bottom", Placement{Left: true, Bottom: true, LeftMargin: 1224, BottomMargin: -24}},
		{"bottom-right", Placement{Right: true, Bottom: true, RightMargin: 24, BottomMargin: -24}},
	}
	for _, tt := range tests {
		t.Run(tt.anchor, func(t *testing.T) {
			input := base
			input.Anchor = tt.anchor
			got, err := Place(input)
			if err != nil || got != tt.want {
				t.Fatalf("Place(%q) = %+v, %v; want %+v", tt.anchor, got, err, tt.want)
			}
		})
	}
	t.Run("negative center and changed size", func(t *testing.T) {
		input := base
		input.Anchor = "center"
		input.MarginX, input.MarginY = -24, 24
		input.WindowWidth, input.WindowHeight = 320, 160
		got, err := Place(input)
		want := Placement{Left: true, Top: true, LeftMargin: 1096, TopMargin: 664}
		if err != nil || got != want {
			t.Fatalf("Place(changed size) = %+v, %v; want %+v", got, err, want)
		}
	})
	t.Run("default output before geometry", func(t *testing.T) {
		got, err := Place(PlacementInput{Anchor: "center", MarginX: 24, MarginY: -24})
		want := Placement{LeftMargin: 24, TopMargin: -24}
		if err != nil || got != want {
			t.Fatalf("Place(default output) = %+v, %v; want %+v", got, err, want)
		}
	})
	t.Run("changed anchor clears old edges", func(t *testing.T) {
		input := base
		input.Anchor = "top-left"
		old, err := Place(input)
		if err != nil {
			t.Fatal(err)
		}
		input.Anchor = "bottom-right"
		current, err := Place(input)
		if err != nil || !old.Left || !old.Top || current.Left || current.Top || !current.Right || !current.Bottom {
			t.Fatalf("anchor transition: old=%+v current=%+v err=%v", old, current, err)
		}
	})
	t.Run("invalid anchor", func(t *testing.T) {
		if _, err := Place(PlacementInput{Anchor: "invalid"}); err == nil {
			t.Fatal("invalid anchor accepted")
		}
	})
}

func TestInvalidWindow(t *testing.T) {
	t.Parallel()
	for _, window := range []*gtk.Window{nil, {}} {
		if err := Init(window); err == nil {
			t.Fatal("Init accepted invalid window")
		}
		if err := Apply(window, nil, Placement{}); err == nil {
			t.Fatal("Apply accepted invalid window")
		}
		if _, err := IsWindow(window); err == nil {
			t.Fatal("IsWindow accepted invalid window")
		}
	}
	if err := Apply(&gtk.Window{}, &gdk.Monitor{}, Placement{}); err == nil {
		t.Fatal("Apply accepted invalid window and monitor")
	}
}
