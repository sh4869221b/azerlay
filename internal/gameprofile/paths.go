package gameprofile

import (
	"path/filepath"

	"github.com/sh4869221b/azerlay/internal/config"
)

func ResolveGamesDir() (string, error) {
	path, err := config.ResolvePath("")
	if err != nil {
		return "", invalid("path", "configuration base unavailable")
	}
	return filepath.Join(filepath.Dir(path), "games"), nil
}
