package input

import "github.com/sh4869221b/azerlay/internal/layout"

type PhysicalControl struct {
	RegionID    string
	SourceID    int
	Down, Known bool
}

// ProjectPhysical projects last-observed button state independently of static
// profile assignments. Unsupported layouts have no physical projection.
func ProjectPhysical(snapshot *Snapshot, definition layout.Definition) []PhysicalControl {
	if definition.Model != "cyborg-ii" || definition.Hand != "left" {
		return nil
	}
	physicalControls := make([]PhysicalControl, 0, len(definition.Controls))
	for _, control := range definition.Controls {
		if control.ID == "stick.main" {
			continue
		}
		physical := PhysicalControl{RegionID: control.ID, SourceID: control.SourceInputID}
		if snapshot.Connected {
			observed := snapshot.Physical(control.ID)
			physical.Down, physical.Known = observed.Down, observed.Known
		}
		physicalControls = append(physicalControls, physical)
	}
	return physicalControls
}
