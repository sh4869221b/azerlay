package live

import (
	"github.com/sh4869221b/azerlay/internal/control"
	"github.com/sh4869221b/azerlay/internal/input"
)

func (c *Coordinator) Status() *control.LiveStatus {
	view := c.Latest()
	if view == nil {
		return nil
	}
	events, draws := c.metrics.Rates()
	dropped := view.Input.Snapshot.InvalidationCount
	status := &control.LiveStatus{EventRate: &events, RenderRate: &draws, DroppedCount: &dropped, Diagnostics: append([]control.Diagnostic{}, view.Diagnostics...)}
	eventMeasured, renderMeasured := c.metrics.Measured()
	if !eventMeasured {
		status.EventRate, status.DroppedCount = nil, nil
	}
	if !renderMeasured {
		status.RenderRate = nil
	}
	if view.Input.Snapshot.Connected && view.Input.Device != "" {
		identity := view.Input.Device
		status.Device = &identity
	}
	if diagnostic := view.Input.Diagnostic; diagnostic != nil {
		status.Diagnostics = append(status.Diagnostics, control.Diagnostic{Code: diagnostic.Code, Stage: diagnostic.Stage, Reason: "Physical input is unavailable."})
	}
	if !view.Input.Snapshot.Connected {
		status.Diagnostics = append(status.Diagnostics, control.Diagnostic{Code: "DEVICE_UNAVAILABLE", Stage: "device", Reason: "Device connection is unavailable."})
	} else if view.Input.Snapshot.Availability != input.Observed {
		status.Diagnostics = append(status.Diagnostics, control.Diagnostic{Code: "INPUT_UNCONFIRMED", Stage: "input", Reason: "Physical controls have no confirmed observations."})
	}
	if reason := view.Input.Snapshot.Reason; reason != "" && view.Input.Snapshot.Connected {
		status.Diagnostics = append(status.Diagnostics, control.Diagnostic{Code: reason, Stage: "input", Reason: "Previous physical observations were invalidated."})
	}
	if view.Profile.Profile == nil {
		status.Diagnostics = append(status.Diagnostics, control.Diagnostic{Code: "PROFILE_UNAVAILABLE", Stage: "profile", Reason: "Active profile is unavailable."})
	}
	return status
}
