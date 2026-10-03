package control

import (
	"time"

	"github.com/sh4869221b/azerlay/internal/config"
)

type ProfileStatus struct {
	Source       string  `json:"source"`
	SourceRef    string  `json:"source_ref"`
	ProfileIndex int     `json:"profile_index"`
	Name         *string `json:"name"`
}

type Diagnostic struct {
	Code   string `json:"code"`
	Stage  string `json:"stage"`
	Reason string `json:"reason"`
}

type ReloadStatus struct {
	RequestGeneration uint64      `json:"request_generation"`
	ConfigGeneration  uint64      `json:"config_generation"`
	ConfigFailure     *Diagnostic `json:"config_failure"`
	WatchFailure      *Diagnostic `json:"watch_failure"`
}

type OverlayStatus struct {
	Mapped             bool `json:"mapped"`
	InputRegionApplied bool `json:"input_region_applied"`
}

// Status reports unavailable capabilities explicitly instead of fabricated metrics.
type Status struct {
	SchemaVersion   int            `json:"schema_version"`
	UptimeSeconds   float64        `json:"uptime_seconds"`
	Visible         bool           `json:"visible"`
	Overlay         *OverlayStatus `json:"overlay,omitempty"`
	ActiveProfile   *ProfileStatus `json:"active_profile"`
	Device          *string        `json:"device"`
	EventNodes      []string       `json:"event_nodes"`
	EventRate       *float64       `json:"event_rate"`
	DroppedCount    *uint64        `json:"dropped_count"`
	ResyncCount     *uint64        `json:"resync_count"`
	RenderRate      *float64       `json:"render_rate"`
	LastReload      ReloadStatus   `json:"last_reload"`
	Generation      uint64         `json:"generation"`
	DegradedReasons []Diagnostic   `json:"degraded_reasons"`
}

func (c *Controller) status() Status {
	reload := c.manager.Snapshot().Status
	c.mu.Lock()
	provider := c.liveStatus
	c.mu.Unlock()
	var live *LiveStatus
	if provider != nil {
		live = provider()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	s := Status{
		SchemaVersion: 1, UptimeSeconds: time.Since(c.started).Seconds(),
		Visible: c.visible, Generation: c.generation, EventNodes: []string{},
		LastReload: ReloadStatus{RequestGeneration: reload.RequestGeneration, ConfigGeneration: reload.ConfigGeneration,
			ConfigFailure: projectDiagnostic(reload.ConfigFailure), WatchFailure: projectDiagnostic(reload.WatchFailure)},
		DegradedReasons: []Diagnostic{
			{Code: "DEVICE_UNAVAILABLE", Stage: "device", Reason: "Device connection is unavailable."},
			{Code: "INPUT_METRICS_UNAVAILABLE", Stage: "input", Reason: "Input metrics are unavailable."},
			{Code: "RENDERER_UNAVAILABLE", Stage: "renderer", Reason: "Overlay renderer is unavailable."},
		},
	}
	if c.active != nil {
		active := c.active.status()
		s.ActiveProfile = &active
	}
	if live != nil {
		s.Device, s.EventRate, s.RenderRate, s.DroppedCount = live.Device, live.EventRate, live.RenderRate, live.DroppedCount
		s.DegradedReasons = append([]Diagnostic{}, live.Diagnostics...)
	}
	if c.overlay != nil {
		copy := *c.overlay
		s.Overlay = &copy
	}
	if c.overlayDiagnostic != nil {
		s.DegradedReasons = append(s.DegradedReasons, *c.overlayDiagnostic)
	}
	if c.selectionFailure != nil {
		s.DegradedReasons = append(s.DegradedReasons, Diagnostic{Code: c.selectionFailure.Code, Stage: c.selectionFailure.Stage, Reason: c.selectionFailure.Summary})
	}
	if c.localFailure != nil {
		s.DegradedReasons = append(s.DegradedReasons, Diagnostic{Code: c.localFailure.Code, Stage: c.localFailure.Stage, Reason: c.localFailure.Summary})
	}
	if c.localWatchFailure != nil {
		s.DegradedReasons = append(s.DegradedReasons, Diagnostic{Code: c.localWatchFailure.Code, Stage: c.localWatchFailure.Stage, Reason: c.localWatchFailure.Summary})
	}
	for _, failure := range []*Diagnostic{s.LastReload.ConfigFailure, s.LastReload.WatchFailure} {
		if failure != nil {
			s.DegradedReasons = append(s.DegradedReasons, *failure)
		}
	}
	return s
}

func projectDiagnostic(d config.Diagnostic) *Diagnostic {
	if d.Code == "" {
		return nil
	}
	return &Diagnostic{Code: d.Code, Stage: d.Stage, Reason: d.Reason}
}
