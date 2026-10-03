package config

import (
	"os"
	"path/filepath"
	"testing"
)

const example = `schema_version = 1
[device]
model = "cyborg-ii"
hand = "left"
serial = ""
auto_reconnect = true
[input]
source = "auto"
refresh_hz = 60
show_ambiguous = true
[profile]
source = "auto"
selected_id = ""
game = "bodycam"
watch = true
[overlay]
monitor = "DP-2"
anchor = "bottom-right"
margin_x = 24
margin_y = 24
scale = 1.0
opacity = 0.88
mode = "normal"
show_profile_name = true
show_status = true
show_unbound = false
[appearance]
theme = "dark"
font_scale = 1.0
high_contrast = false
[diagnostics]
log_level = "info"
include_bindings = false
`

func configFile(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConfigLoad(t *testing.T) {
	t.Parallel()
	want := Config{
		SchemaVersion: 1,
		Device:        Device{Model: "cyborg-ii", Hand: "left", AutoReconnect: true},
		Input:         Input{Source: "auto", RefreshHz: 60, ShowAmbiguous: true},
		Profile:       Profile{Source: "auto", Game: "bodycam", Watch: true},
		Overlay:       Overlay{Anchor: "bottom-right", MarginX: 24, MarginY: 24, Scale: 1, Opacity: .88, Mode: "normal", ShowProfileName: true, ShowStatus: true},
		Appearance:    Appearance{Theme: "dark", FontScale: 1},
		Diagnostics:   Diagnostics{LogLevel: "info"},
	}
	for _, tc := range []struct {
		name, text string
		want       Config
	}{
		{"defaults", "schema_version = 1", want},
		{"local selection", "schema_version=1\n[profile]\nsource='local'\nlocal_store_path='../userData'\nlocal_device='device-1'\nlocal_profile_file='profile_local.json'", func() Config {
			c := want
			c.Profile.Source = "local"
			c.Profile.LocalStorePath = "../userData"
			c.Profile.LocalDevice = "device-1"
			c.Profile.LocalProfileFile = "profile_local.json"
			return c
		}()},
		{"root without selection", "schema_version=1\n[profile]\nlocal_store_path='userData'", func() Config { c := want; c.Profile.LocalStorePath = "userData"; return c }()},
		{"example", example, func() Config { c := want; c.Overlay.Monitor = "DP-2"; return c }()},
		{"explicit zero values", `schema_version=1
[device]
auto_reconnect=false
[input]
show_ambiguous=false
[profile]
game=""
watch=false
[overlay]
monitor=""
margin_x=0
margin_y=0
opacity=0.0
show_profile_name=false
show_status=false
`, func() Config {
			c := want
			c.Device.AutoReconnect = false
			c.Input.ShowAmbiguous = false
			c.Profile.Game = ""
			c.Profile.Watch = false
			c.Overlay.MarginX = 0
			c.Overlay.MarginY = 0
			c.Overlay.Opacity = 0
			c.Overlay.ShowProfileName = false
			c.Overlay.ShowStatus = false
			return c
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, warnings, err := Load(configFile(t, tc.text))
			if err != nil || len(warnings) != 0 || got != tc.want {
				t.Fatalf("Load() = %+v, %v, %v; want %+v", got, warnings, err, tc.want)
			}
		})
	}
}
