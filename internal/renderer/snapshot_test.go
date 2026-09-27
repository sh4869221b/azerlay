package renderer

import (
	"reflect"
	"testing"

	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/profile"
)

func snapshotFixture() layout.Definition {
	return layout.Definition{
		Applicability: layout.Applicability{HardwareRevision: stringPointer("synthetic")},
		Controls: []layout.Control{
			{ID: "grid.c1.r1", Shape: layout.Shape{Type: "group", Children: []layout.Shape{{Type: "polygon", Points: []layout.Point{{X: 1, Y: 2}}}, {Type: "path", Commands: []layout.PathCommand{{Op: "M", Points: []layout.Point{{X: 3, Y: 4}}}}}}}},
			{ID: "grid.c1.r2"},
			{ID: "stick.main"},
		},
		Decorations: []layout.Shape{{Type: "group", Children: []layout.Shape{{Type: "line", Points: []layout.Point{{X: 5, Y: 6}}, Commands: []layout.PathCommand{{Op: "L", Points: []layout.Point{{X: 7, Y: 8}}}}}}}},
	}
}

func stringPointer(value string) *string { return &value }

func TestSnapshotOwnership(t *testing.T) {
	t.Parallel()
	definition := snapshotFixture()
	content := Content{
		ProfileName: "Synthetic profile",
		Status:      StatusReloadFailed,
		Controls: map[string]Control{
			"grid.c1.r1":     {Known: true, Down: true, Assignments: []Assignment{{Trigger: profile.TriggerLong, Kind: profile.BindingMacro, Label: "Synthetic action", BindingDisplay: "Macro", Ambiguous: true}}},
			"outside.layout": {Known: true, Down: true},
		},
	}
	wantDefinition := snapshotFixture()
	wantAssignment := content.Controls["grid.c1.r1"].Assignments[0]
	snapshot := NewSnapshot(definition, content)

	*definition.Applicability.HardwareRevision = "changed"
	definition.Controls[0].ID = "changed"
	definition.Controls[0].Shape.Children[0].Points[0].X = 100
	definition.Controls[0].Shape.Children[1].Commands[0].Points[0].Y = 100
	definition.Decorations[0].Children[0].Points[0].X = 100
	definition.Decorations[0].Children[0].Commands[0].Points[0].Y = 100
	control := content.Controls["grid.c1.r1"]
	control.Assignments[0].Label = "changed"
	content.Controls["grid.c1.r1"] = Control{}
	delete(content.Controls, "outside.layout")
	content.Status = StatusDisconnected

	if !reflect.DeepEqual(snapshot.definition, wantDefinition) {
		t.Fatalf("snapshot layout changed with source: %+v", snapshot.definition)
	}
	if got := snapshot.content.Controls["grid.c1.r1"]; !got.Known || !got.Down || !reflect.DeepEqual(got.Assignments, []Assignment{wantAssignment}) {
		t.Fatalf("snapshot content changed with source: %+v", got)
	}
	if len(snapshot.content.Controls) != len(wantDefinition.Controls) {
		t.Fatalf("out-of-layout control retained: %d", len(snapshot.content.Controls))
	}
	if snapshot.content.Status != StatusReloadFailed || snapshot.content.ProfileName != "Synthetic profile" || snapshot.content.Status.message() != "Reload failed" {
		t.Fatalf("status or supplied content changed: %+v", snapshot.content)
	}
}

func TestSnapshotPhysicalAndAssignment(t *testing.T) {
	t.Parallel()
	assignments := []Assignment{
		{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Label: "Single", BindingDisplay: "W"},
		{Trigger: profile.TriggerLong, Kind: profile.BindingKeyboard, Label: "Hold", BindingDisplay: "U"},
		{Trigger: profile.TriggerDouble, Kind: profile.BindingMacro, Label: "Double", BindingDisplay: "Macro", Ambiguous: true},
		{Trigger: profile.TriggerUnknown, Kind: profile.BindingUnbound, Label: "Unbound", BindingDisplay: "-"},
	}
	content := Content{Controls: map[string]Control{"grid.c1.r1": {Known: true, Down: true, Assignments: assignments}}}
	pressed := NewSnapshot(snapshotFixture(), content)
	content.Controls["grid.c1.r1"] = Control{Known: true, Down: false, Assignments: assignments}
	released := NewSnapshot(snapshotFixture(), content)
	for _, snapshot := range []*OverlaySnapshot{pressed, released} {
		if got := snapshot.content.Controls["grid.c1.r1"].Assignments; !reflect.DeepEqual(got, assignments) {
			t.Fatalf("physical observation changed static assignments: %+v", got)
		}
	}
	if !pressed.content.Controls["grid.c1.r1"].Down || released.content.Controls["grid.c1.r1"].Down {
		t.Fatal("physical observations were not kept separately")
	}
}

func TestSnapshotMissingControl(t *testing.T) {
	t.Parallel()
	snapshot := NewSnapshot(snapshotFixture(), Content{})
	control, ok := snapshot.content.Controls["grid.c1.r2"]
	if !ok || control.Known || control.Down || !reflect.DeepEqual(control.Assignments, []Assignment{{Trigger: profile.TriggerUnknown, Kind: profile.BindingUnknown}}) {
		t.Fatalf("missing control was not unknown: %+v, present=%t", control, ok)
	}
	if len(snapshot.definition.Controls) != 3 {
		t.Fatal("missing content removed layout geometry")
	}
}

func TestSnapshotUnknownObservation(t *testing.T) {
	t.Parallel()
	snapshot := NewSnapshot(snapshotFixture(), Content{Controls: map[string]Control{"grid.c1.r1": {Down: true, Assignments: []Assignment{{Kind: profile.BindingUnbound}}}}})
	control := snapshot.content.Controls["grid.c1.r1"]
	if control.Known || control.Down || control.Assignments[0].Kind != profile.BindingUnbound {
		t.Fatalf("unknown observation became pressed or altered assignment: %+v", control)
	}
}

func TestSnapshotStaticStick(t *testing.T) {
	t.Parallel()
	snapshot := NewSnapshot(snapshotFixture(), Content{Controls: map[string]Control{"stick.main": {Known: true, Down: true, Assignments: []Assignment{{Kind: profile.BindingStick, Label: "Static stick"}}}}})
	stick := snapshot.content.Controls["stick.main"]
	if stick.Known || stick.Down || stick.Assignments[0].Kind != profile.BindingStick || stick.Assignments[0].Label != "Static stick" {
		t.Fatalf("stick gained physical observation or lost static content: %+v", stick)
	}
}
