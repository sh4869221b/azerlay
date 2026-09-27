package gameprofile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPaths(t *testing.T) {
	for _, tc := range []struct {
		name, xdg, home, want string
		invalid               bool
	}{
		{"xdg", t.TempDir(), "", "xdg", false},
		{"home", "relative", t.TempDir(), "home", false},
		{"invalid", "relative", "relative", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", tc.xdg)
			t.Setenv("HOME", tc.home)
			got, err := ResolveGamesDir()
			if tc.invalid {
				if !hasStage(err, "path") {
					t.Fatalf("ResolveGamesDir: %q, %v", got, err)
				}
				return
			}
			base := tc.xdg
			if tc.want == "home" {
				base = filepath.Join(tc.home, ".config")
			}
			want := filepath.Join(base, "azerlay", "games")
			if err != nil || got != want {
				t.Fatalf("ResolveGamesDir: %q, %v; want %q", got, err, want)
			}
			if _, err := os.Stat(got); !os.IsNotExist(err) {
				t.Fatalf("ResolveGamesDir wrote directory: %v", err)
			}
			catalog, err := Load("")
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := catalog.Lookup("generic"); !ok {
				t.Fatal("default load omitted embedded generic")
			}
			if _, err := os.Stat(got); !os.IsNotExist(err) {
				t.Fatalf("Load wrote directory: %v", err)
			}
		})
	}
}
