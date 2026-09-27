package input

import (
	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/matching"
	"github.com/sh4869221b/azerlay/internal/profile"
	"testing"
)

func TestPhysicalProjectionIndependentOfBindings(t *testing.T) {
	definition, err := layout.LoadEmbedded("cyborg-ii", "left")
	if err != nil {
		t.Fatal(err)
	}
	context := PhysicalContext{Context: matching.Context{Model: definition.Model, Hand: definition.Hand, Applicability: definition.Applicability}, SoftwareMode: true}
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
	index := matching.Build(selected, profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export"}, definition, context.Context)
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
		state := ProjectMatching(snapshot, index, context)
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
		for _, change := range []func(*PhysicalContext){func(c *PhysicalContext) { c.Hand = "right" }, func(c *PhysicalContext) { c.Applicability.SoftwareRelease = "2.0.3" }, func(c *PhysicalContext) { c.Applicability.DisplayedFirmware = "112" }, func(c *PhysicalContext) { c.Applicability.Mode = "xbox-stick" }, func(c *PhysicalContext) { c.SoftwareMode = false }} {
			invalid := context
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
		if stopped := ProjectMatching(r.stop(), index, context); stopped.Connected {
			t.Fatal("disconnect retained")
		}
	}
}
