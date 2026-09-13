package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigPaths(t *testing.T) {
	home := t.TempDir()
	xdg := t.TempDir()
	for _, tc := range []struct {
		name, explicit, xdg, home, want string
		invalid                         bool
	}{
		{"explicit", "sub/../config.toml", "relative", "", "config.toml", false},
		{"xdg", "", xdg, "", filepath.Join(xdg, "azerlay/config.toml"), false},
		{"home", "", "", home, filepath.Join(home, ".config/azerlay/config.toml"), false},
		{"relative xdg", "", "relative", home, filepath.Join(home, ".config/azerlay/config.toml"), false},
		{"missing bases", "", "", "", "", true},
		{"relative bases", "", "relative", "relative", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", tc.xdg)
			t.Setenv("HOME", tc.home)
			got, err := ResolvePath(tc.explicit)
			if tc.invalid {
				var configErr *Error
				if !errors.As(err, &configErr) || configErr.Code != ERR_CONFIG_INVALID || configErr.Stage != "path" {
					t.Fatalf("ResolvePath() = %q, %v", got, err)
				}
				return
			}
			want, absErr := filepath.Abs(tc.want)
			if absErr != nil {
				t.Fatal(absErr)
			}
			if err != nil || got != want {
				t.Fatalf("ResolvePath() = %q, %v; want %q", got, err, want)
			}
		})
	}
	t.Run("load default", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", xdg)
		dir := filepath.Join(xdg, "azerlay")
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("schema_version=1"), 0600); err != nil {
			t.Fatal(err)
		}
		got, _, err := Load("")
		if err != nil || got.SchemaVersion != 1 {
			t.Fatalf("Load(default) = %v, %v", got, err)
		}
	})
}
