package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigInvalid(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, text, stage string }{
		{"syntax", "schema_version = [", "decode"},
		{"schema missing", "", "schema"},
		{"schema unknown", "schema_version = 2", "schema"},
		{"schema type", "schema_version = 1.0", "decode"},
		{"bool type", "schema_version=1\n[device]\nauto_reconnect=0", "decode"},
		{"string type", "schema_version=1\n[profile]\ngame=5", "decode"},
		{"model", "schema_version=1\n[device]\nmodel='other'", "validation"},
		{"hand", "schema_version=1\n[device]\nhand='other'", "validation"},
		{"input source", "schema_version=1\n[input]\nsource='other'", "validation"},
		{"refresh rate", "schema_version=1\n[input]\nrefresh_hz=59", "validation"},
		{"profile source", "schema_version=1\n[profile]\nsource='other'", "validation"},
		{"local half device", "schema_version=1\n[profile]\nlocal_device='device'", "validation"},
		{"local half file", "schema_version=1\n[profile]\nlocal_profile_file='profile_a.json'", "validation"},
		{"local parent", "schema_version=1\n[profile]\nlocal_device='..'\nlocal_profile_file='profile_a.json'", "validation"},
		{"local separator", "schema_version=1\n[profile]\nlocal_device='device'\nlocal_profile_file='nested/profile_a.json'", "validation"},
		{"local backslash", "schema_version=1\n[profile]\nlocal_device='device\\other'\nlocal_profile_file='profile_a.json'", "validation"},
		{"local absolute", "schema_version=1\n[profile]\nlocal_device='/device'\nlocal_profile_file='profile_a.json'", "validation"},
		{"local nul", "schema_version=1\n[profile]\nlocal_device=\"dev\\u0000ice\"\nlocal_profile_file='profile_a.json'", "validation"},
		{"local selected id", "schema_version=1\n[profile]\nsource='local'\nselected_id='id'", "validation"},
		{"auto conflicting selectors", "schema_version=1\n[profile]\nselected_id='id'\nlocal_device='device'\nlocal_profile_file='profile_a.json'", "validation"},
		{"imported local ref", "schema_version=1\n[profile]\nsource='imported'\nlocal_device='device'\nlocal_profile_file='profile_a.json'", "validation"},
		{"anchor", "schema_version=1\n[overlay]\nanchor='other'", "validation"},
		{"mode", "schema_version=1\n[overlay]\nmode='other'", "validation"},
		{"theme", "schema_version=1\n[appearance]\ntheme='other'", "validation"},
		{"log level", "schema_version=1\n[diagnostics]\nlog_level='other'", "validation"},
		{"margin lower", "schema_version=1\n[overlay]\nmargin_x=-16385", "validation"},
		{"margin upper", "schema_version=1\n[overlay]\nmargin_y=16385", "validation"},
		{"margin type", "schema_version=1\n[overlay]\nmargin_x=1.0", "decode"},
		{"scale lower", "schema_version=1\n[overlay]\nscale=0.24", "validation"},
		{"scale upper", "schema_version=1\n[overlay]\nscale=4.01", "validation"},
		{"scale nonfinite", "schema_version=1\n[overlay]\nscale=nan", "validation"},
		{"opacity lower", "schema_version=1\n[overlay]\nopacity=-0.01", "validation"},
		{"opacity upper", "schema_version=1\n[overlay]\nopacity=1.01", "validation"},
		{"opacity nonfinite", "schema_version=1\n[overlay]\nopacity=inf", "validation"},
		{"font lower", "schema_version=1\n[appearance]\nfont_scale=0.24", "validation"},
		{"font upper", "schema_version=1\n[appearance]\nfont_scale=4.01", "validation"},
		{"font nonfinite", "schema_version=1\n[appearance]\nfont_scale=-inf", "validation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := Load(configFile(t, tc.text))
			var configErr *Error
			if !errors.As(err, &configErr) || configErr.Code != ERR_CONFIG_INVALID || configErr.Stage != tc.stage || got != (Config{}) {
				t.Fatalf("Load() = %+v, %v; want invalid %s", got, err, tc.stage)
			}
		})
	}
	for _, name := range []string{"config.toml", "absent/config.toml"} {
		t.Run(name, func(t *testing.T) {
			got, _, err := Load(filepath.Join(t.TempDir(), name))
			var configErr *Error
			if !errors.As(err, &configErr) || configErr.Code != ERR_CONFIG_NOT_FOUND || !errors.Is(err, os.ErrNotExist) || got != (Config{}) {
				t.Fatalf("missing Load() = %v, %v", got, err)
			}
		})
	}
	t.Run("io failure", func(t *testing.T) {
		_, _, err := Load(t.TempDir())
		var configErr *Error
		if !errors.As(err, &configErr) || configErr.Code != ERR_CONFIG_INVALID || configErr.Stage != "read" {
			t.Fatalf("directory Load() = %v", err)
		}
	})
}
