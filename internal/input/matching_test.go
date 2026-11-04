package input

import (
	"testing"

	"github.com/sh4869221b/azerlay/internal/layout"
)

func TestPhysicalProjection(t *testing.T) {
	t.Parallel()
	definition, err := layout.LoadEmbedded("cyborg-ii", "left")
	if err != nil {
		t.Fatal(err)
	}
	d, err := newPhysicalDecoder()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []byte{4, 8, 3, 2} {
		r := newReducer(Generations{7, 9})
		event, _, err := d.decode(physicalBytes(id, 1, 1))
		if err != nil {
			t.Fatal(err)
		}
		snapshot := r.apply(event)
		physical := ProjectPhysical(snapshot, definition)
		if len(physical) != 30 {
			t.Fatalf("physical controls: %d; want 30", len(physical))
		}
		known := 0
		for _, control := range physical {
			if control.Known {
				known++
				if control.SourceID != int(id) || !control.Down {
					t.Fatal("binding inferred physical control")
				}
			}
		}
		if known != 1 {
			t.Fatal("unobserved state fabricated")
		}
		disconnected := *snapshot
		disconnected.Connected = false
		for _, control := range ProjectPhysical(&disconnected, definition) {
			if control.Known || control.Down {
				t.Fatal("disconnected snapshot retained physical knowledge")
			}
		}
		for _, change := range []func(*layout.Definition){func(d *layout.Definition) { d.Hand = "right" }, func(d *layout.Definition) { d.Model = "other" }} {
			invalid := definition
			change(&invalid)
			if physical := ProjectPhysical(snapshot, invalid); len(physical) != 0 {
				t.Fatal("unsupported scope projected physical controls")
			}
		}
		release, _, err := d.decode(physicalBytes(id, 0, 2))
		if err != nil {
			t.Fatal(err)
		}
		for _, control := range ProjectPhysical(r.apply(release), definition) {
			if control.SourceID == int(id) && (!control.Known || control.Down) {
				t.Fatal("release lost")
			}
		}
		for _, unknown := range []*Snapshot{r.invalidate(ERR_INPUT_EVENT), r.stop()} {
			for _, control := range ProjectPhysical(unknown, definition) {
				if control.Known || control.Down {
					t.Fatal("invalidation retained physical knowledge")
				}
			}
		}
	}
}
