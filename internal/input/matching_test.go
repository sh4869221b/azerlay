package input

import (
	"testing"

	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/matching"
	"github.com/sh4869221b/azerlay/internal/profile"
)

func TestPhysicalProjectionIndependentOfBindings(t *testing.T) {
	definition, err := layout.LoadEmbedded("cyborg-ii", "left")
	if err != nil {
		t.Fatal(err)
	}
	var selected profile.Profile
	for i, id := range []int{4, 8, 3, 2} {
		kind := profile.BindingKeyboard
		if i == 2 {
			kind = profile.BindingUnbound
		}
		if i == 3 {
			kind = profile.BindingUnknown
		}
		selected.Controls = append(selected.Controls, profile.ControlBinding{SourceIdentity: profile.SourceIdentity{InputID: &id}, Bindings: []profile.TriggerBinding{{Kind: kind, Trigger: profile.TriggerSingle, Actions: []profile.Action{{Code: profile.KEY_U}}}}})
	}
	index := matching.Build(selected, profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export"}, definition)
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
		state := ProjectMatching(snapshot, index, definition)
		if !state.ScopeSupported || len(state.Physical) != 30 || len(state.Controls) != 4 || len(state.Outputs) != 1 || len(state.Outputs[0].Candidates) != 2 || !state.Outputs[0].Ambiguous {
			t.Fatalf("static candidates lost: %+v", state)
		}
		known := 0
		for _, control := range state.Physical {
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
		for _, static := range []matching.Index{matching.Build(selected, profile.SourceMetadata{}, definition), {}} {
			independent := ProjectMatching(snapshot, static, definition)
			if !independent.ScopeSupported || len(independent.Physical) != 30 {
				t.Fatal("static scope suppressed physical projection")
			}
			known := 0
			for _, control := range independent.Physical {
				if control.Known {
					known++
					if control.SourceID != int(id) || !control.Down {
						t.Fatal("static scope changed raw observation")
					}
				}
			}
			if known != 1 {
				t.Fatal("missing profile suppressed raw observation")
			}
		}
		for _, change := range []func(*layout.Definition){func(d *layout.Definition) { d.Hand = "right" }, func(d *layout.Definition) { d.Model = "other" }} {
			invalid := definition
			change(&invalid)
			unsupported := ProjectMatching(snapshot, index, invalid)
			if unsupported.ScopeSupported {
				t.Fatal("unsupported scope")
			}
			for _, control := range unsupported.Physical {
				if control.Known {
					t.Fatal("unsupported known")
				}
			}
		}
		release, _, err := d.decode(physicalBytes(id, 0, 2))
		if err != nil {
			t.Fatal(err)
		}
		for _, control := range ProjectMatching(r.apply(release), index, definition).Physical {
			if control.SourceID == int(id) && (!control.Known || control.Down) {
				t.Fatal("release lost")
			}
		}
		for _, unknown := range []*Snapshot{r.invalidate(ERR_INPUT_EVENT), r.stop()} {
			for _, control := range ProjectMatching(unknown, index, definition).Physical {
				if control.Known || control.Down {
					t.Fatal("invalidation retained physical knowledge")
				}
			}
		}
		if ProjectMatching(r.snapshot(), index, definition).Connected {
			t.Fatal("disconnect retained connection")
		}
	}
}
