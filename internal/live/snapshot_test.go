package live

import (
	"strconv"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/control"
	"github.com/sh4869221b/azerlay/internal/gameprofile"
	"github.com/sh4869221b/azerlay/internal/input"
	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/renderer"
)

func assemblyFixture(t *testing.T) AssemblyState {
	t.Helper()
	definition, err := layout.LoadEmbedded("cyborg-ii", "left")
	if err != nil {
		t.Fatal(err)
	}
	id, other, unresolved := 4, 3, 999
	selected := &profile.Profile{Controls: []profile.ControlBinding{
		{SourceIdentity: profile.SourceIdentity{InputID: &id}, Bindings: []profile.TriggerBinding{{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Actions: []profile.Action{{Kind: profile.ActionKeyboard, Code: "KEY_W"}}}, {Trigger: profile.TriggerLong, Kind: profile.BindingUnknown}}},
		{SourceIdentity: profile.SourceIdentity{InputID: &other}, Bindings: []profile.TriggerBinding{{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Actions: []profile.Action{{Kind: profile.ActionKeyboard, Code: "KEY_W"}}}}},
		{SourceIdentity: profile.SourceIdentity{InputID: &unresolved}, Bindings: []profile.TriggerBinding{{Kind: profile.BindingUnbound}}},
	}}
	return AssemblyState{Profile: &control.ProfileSnapshot{Generation: 1, Source: profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-local-json"}, Profile: selected}, Definition: definition, LayoutGeneration: 1, LabelsGeneration: 1, Input: &input.Snapshot{Connected: true}, Game: gameprofile.Definition{Controls: map[gameprofile.ControlKey]string{{InputID: 4, Trigger: profile.TriggerSingle}: "Synthetic action"}}}
}

func TestAssembly(t *testing.T) {
	t.Parallel()
	t.Run("labels-triggers-ambiguity-and-unresolved", func(t *testing.T) {
		state := assemblyFixture(t)
		var assembly Assembly
		assembly.Build(state)
		assignments := assembly.content.Controls["grid.c1.r1"].Assignments
		if len(assignments) != 2 || assignments[0].Label != "Synthetic action" || !assignments[0].Ambiguous || assignments[1].Kind != profile.BindingUnknown {
			t.Fatalf("assignments: %+v", assignments)
		}
		if _, exists := assembly.content.Controls[""]; exists {
			t.Fatal("unresolved assignment guessed a region")
		}
	})
	t.Run("same-observation-retains-pointer", func(t *testing.T) {
		state := assemblyFixture(t)
		var assembly Assembly
		before := assembly.Build(state)
		state.Input = &input.Snapshot{Connected: true, Sequence: 10, EventCount: 10, ReadAt: time.Unix(10, 0)}
		if assembly.Build(state) != before {
			t.Fatal("counters changed render identity")
		}
		state.Game = gameprofile.Definition{}
		state.LabelsGeneration++
		if assembly.Build(state) == before || assembly.content.Controls["grid.c1.r1"].Assignments[0].Label == "Synthetic action" {
			t.Fatal("changed labels were not applied")
		}
	})
	t.Run("missing-profile-layout-and-reload", func(t *testing.T) {
		state := assemblyFixture(t)
		state.Profile = &control.ProfileSnapshot{Generation: 2}
		var assembly Assembly
		before := assembly.Build(state)
		if assembly.content.Status != renderer.StatusProfileMissing || len(assembly.content.Controls) != 30 {
			t.Fatal("missing profile removed physical layout")
		}
		state.LayoutGeneration++
		if assembly.Build(state) == before {
			t.Fatal("layout generation reused geometry")
		}
		state.ReloadFailed = true
		assembly.Build(state)
		if assembly.content.Status != renderer.StatusReloadFailed {
			t.Fatal("reload failure missing")
		}
	})
}

func TestCoalescer(t *testing.T) {
	t.Parallel()
	for _, hz := range []int{30, 60, 120} {
		t.Run(strconv.Itoa(hz)+"Hz", func(t *testing.T) {
			var coalescer Coalescer
			now := time.Unix(100, 0)
			if coalescer.Delay(now, hz) != 0 {
				t.Fatal("initial render delayed")
			}
			coalescer.Applied(now)
			interval := time.Second / time.Duration(hz)
			if delay := coalescer.Delay(now, hz); delay < interval || delay-interval >= time.Millisecond {
				t.Fatalf("cap rounding: %v", delay)
			}
			if coalescer.Delay(now.Add(interval), hz) != 0 {
				t.Fatal("deadline render delayed")
			}
			coalescer.Applied(now.Add(interval))
			if coalescer.Delay(now.Add(interval+time.Millisecond), hz) <= 0 {
				t.Fatal("backlog bypassed cap")
			}
		})
	}
}
