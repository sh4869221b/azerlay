package control

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

// Status reports unavailable capabilities explicitly instead of fabricated metrics.
type Status struct {
	SchemaVersion   int            `json:"schema_version"`
	UptimeSeconds   float64        `json:"uptime_seconds"`
	Visible         bool           `json:"visible"`
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
