//go:build linux

package input

import (
	"bytes"
	"context"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profilesource"
)

func TestStickPipeline(t *testing.T) {
	for _, mode := range []string{"keyboard", "xbox", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			fixture := mode
			if mode == "unknown" {
				fixture = "xbox"
			}
			export, err := os.ReadFile("../profileadapter/testdata/stick-" + fixture + ".json")
			if err != nil {
				t.Fatal(err)
			}
			if mode == "unknown" {
				export = bytes.Replace(export, []byte(`"21"`), []byte(`"3"`), 1)
			}
			prepared, err := profilesource.PrepareReader(bytes.NewReader(export), profile.SourceMetadata{
				SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export",
			})
			if err != nil {
				t.Fatal(err)
			}
			home := t.TempDir()
			selection, err := profilesource.NewImportedSource(home).Import(context.Background(), prepared, 1, profilesource.Origin{Kind: "stdin"})
			if err != nil {
				t.Fatal(err)
			}
			store := profilesource.NewImportedSource(home)
			selected, err := store.Selected(context.Background())
			if err != nil || selected != selection {
				t.Fatalf("saved selection = %+v, %v", selected, err)
			}
			loaded, err := store.Load(context.Background(), selected.Source)
			if err != nil || loaded == nil || !reflect.DeepEqual(*loaded, prepared.Bundle()) {
				t.Fatalf("reloaded profile differs: %v", err)
			}
			binding := loaded.Profiles[selected.ProfileIndex-1].Controls[0].Bindings[0]
			if mode == "unknown" {
				if binding.Kind != profile.BindingUnknown || binding.Stick != nil || binding.Unknown == nil {
					t.Fatalf("unsupported mode became projectable: %+v", binding)
				}
				return
			}
			if binding.Kind != profile.BindingStick || binding.Stick == nil || string(binding.Stick.Mode) != mode {
				t.Fatalf("imported stick = %+v", binding)
			}
			generations := Generations{Device: 3, Profile: 7}
			s := newIntegrationSessionWithAxes(t, 2, generations, map[uint16]AxisInfo{
				0: {Minimum: 0, Maximum: 200, Flat: 20, Fuzz: 2},
				1: {Minimum: -100, Maximum: 100, Flat: 20},
			})
			source := StickSource{
				Xbox:     XboxStickSource{Node: 1, X: 0, Y: 1},
				Keyboard: KeyboardStickSource{Up: 1, Right: 1, Down: 1, Left: 1},
			}
			project := func(snapshot *Snapshot) StickState { return ProjectStick(snapshot, *binding.Stick, source) }
			initial := project(s.Latest())
			if initial.Known || !initial.Connected || initial.Mode != binding.Stick.Mode || initial.Generations != generations {
				t.Fatalf("initial stick = %+v", initial)
			}
			var retained *Snapshot
			switch mode {
			case "xbox":
				s.send(t, 1, Event{Type: EV_ABS, Code: 0, Value: 160}, Event{Type: EV_ABS, Code: 1, Value: -100})
				if got := project(s.Latest()); got != initial {
					t.Fatalf("pending axes changed projection: %+v", got)
				}
				other := s.send(t, 0, Event{Type: EV_ABS, Code: 0, Value: 0}, Event{Type: EV_ABS, Code: 1, Value: 100}, Event{Type: EV_SYN})
				if got := project(other); got.Known || got.AxisX.Known || got.AxisY.Known {
					t.Fatalf("other node report exposed selected axes: %+v", got)
				}
				retained = s.send(t, 1, Event{Type: EV_SYN})
				got := project(retained)
				if !got.Known || got.AxisX != (AxisState{Raw: 160, Normalized: 0.5, Known: true, Valid: true}) ||
					got.AxisY != (AxisState{Raw: -100, Normalized: -1, Known: true, Valid: true}) ||
					got.X != 0.5 || got.Y != -1 || got.Intensity != 1 || math.Abs(got.DirectionX-1/math.Sqrt(5)) > 1e-12 || math.Abs(got.DirectionY+2/math.Sqrt(5)) > 1e-12 {
					t.Fatalf("committed Xbox projection = %+v", got)
				}
				other = s.send(t, 0, Event{Type: EV_ABS, Code: 0, Value: 200}, Event{Type: EV_ABS, Code: 1, Value: 0}, Event{Type: EV_SYN})
				changed := project(other)
				changed.Sequence = got.Sequence
				if changed != got {
					t.Fatalf("other node changed selected stick: %+v", changed)
				}
			case "keyboard":
				neutral := s.send(t, 1,
					Event{Type: EV_KEY, Code: 17}, Event{Type: EV_KEY, Code: 32},
					Event{Type: EV_KEY, Code: 31}, Event{Type: EV_KEY, Code: 30}, Event{Type: EV_SYN})
				if got := project(neutral); !got.Known || got.X != 0 || got.Y != 0 || got.Intensity != 0 {
					t.Fatalf("explicit releases did not establish neutral: %+v", got)
				}
				s.send(t, 1, Event{Type: EV_KEY, Code: 17, Value: 1}, Event{Type: EV_KEY, Code: 32, Value: 1})
				if got := project(s.Latest()); got != project(neutral) {
					t.Fatalf("pending keys changed projection: %+v", got)
				}
				retained = s.send(t, 1, Event{Type: EV_SYN})
				got := project(retained)
				if !got.Known || !got.Up.Down || !got.Right.Down || got.Down.Down || got.Left.Down ||
					got.X != 1 || got.Y != -1 || got.Intensity != 1 || math.Abs(got.DirectionX-1/math.Sqrt2) > 1e-12 || math.Abs(got.DirectionY+1/math.Sqrt2) > 1e-12 {
					t.Fatalf("W+D projection = %+v", got)
				}
				repeat := project(s.send(t, 1, Event{Type: EV_KEY, Code: 17, Value: 2}, Event{Type: EV_SYN}))
				repeat.Sequence = got.Sequence
				if repeat != got {
					t.Fatalf("repeat changed stick: %+v", repeat)
				}
				released := project(s.send(t, 1, Event{Type: EV_KEY, Code: 17}, Event{Type: EV_KEY, Code: 32}, Event{Type: EV_SYN}))
				if !released.Known || released.Up.Down || released.Right.Down || released.Intensity != 0 || released.X != 0 || released.Y != 0 {
					t.Fatalf("release projection = %+v", released)
				}
			}
			before := project(retained)
			sequence := s.Latest().Sequence
			if err := s.writers[1].Close(); err != nil {
				t.Fatal(err)
			}
			assertIntegrationStopped(t, s, ERR_INPUT_READ, sequence+1, generations)
			terminal := project(s.Latest())
			if terminal != (StickState{Mode: binding.Stick.Mode, Sequence: sequence + 1, Generations: generations}) {
				t.Fatalf("terminal retained live projection: %+v", terminal)
			}
			if project(retained) != before {
				t.Fatal("disconnect mutated retained projection")
			}
		})
	}
}
