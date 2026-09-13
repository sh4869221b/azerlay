package config

type Config struct {
	SchemaVersion int         `toml:"schema_version"`
	Device        Device      `toml:"device"`
	Input         Input       `toml:"input"`
	Profile       Profile     `toml:"profile"`
	Overlay       Overlay     `toml:"overlay"`
	Appearance    Appearance  `toml:"appearance"`
	Diagnostics   Diagnostics `toml:"diagnostics"`
}

type Device struct {
	Model         string `toml:"model"`
	Hand          string `toml:"hand"`
	Serial        string `toml:"serial"`
	AutoReconnect bool   `toml:"auto_reconnect"`
}
type Input struct {
	Source        string `toml:"source"`
	RefreshHz     int    `toml:"refresh_hz"`
	ShowAmbiguous bool   `toml:"show_ambiguous"`
}
type Profile struct {
	Source     string `toml:"source"`
	SelectedID string `toml:"selected_id"`
	Game       string `toml:"game"`
	Watch      bool   `toml:"watch"`
}
type Overlay struct {
	Monitor         string  `toml:"monitor"`
	Anchor          string  `toml:"anchor"`
	MarginX         int     `toml:"margin_x"`
	MarginY         int     `toml:"margin_y"`
	Scale           float64 `toml:"scale"`
	Opacity         float64 `toml:"opacity"`
	Mode            string  `toml:"mode"`
	ShowProfileName bool    `toml:"show_profile_name"`
	ShowStatus      bool    `toml:"show_status"`
	ShowUnbound     bool    `toml:"show_unbound"`
}
type Appearance struct {
	Theme        string  `toml:"theme"`
	FontScale    float64 `toml:"font_scale"`
	HighContrast bool    `toml:"high_contrast"`
}
type Diagnostics struct {
	LogLevel        string `toml:"log_level"`
	IncludeBindings bool   `toml:"include_bindings"`
}

// defaults deliberately leaves SchemaVersion zero: every file must specify it.
func defaults() Config {
	return Config{
		Device:      Device{Model: "cyborg-ii", Hand: "left", AutoReconnect: true},
		Input:       Input{Source: "auto", RefreshHz: 60, ShowAmbiguous: true},
		Profile:     Profile{Source: "auto", Game: "bodycam", Watch: true},
		Overlay:     Overlay{Anchor: "bottom-right", MarginX: 24, MarginY: 24, Scale: 1, Opacity: .88, Mode: "normal", ShowProfileName: true, ShowStatus: true},
		Appearance:  Appearance{Theme: "dark", FontScale: 1},
		Diagnostics: Diagnostics{LogLevel: "info"},
	}
}
